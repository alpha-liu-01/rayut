#include "controller.h"

#include <algorithm>

#include "qrdraw.h"
#include "qrscan.h"

#include <QBuffer>
#include <QClipboard>
#include <QCoreApplication>
#include <QGuiApplication>
#include <QImage>
#include <QImageReader>

#include <cstdlib>
#include <cstring>
#include <QDir>
#include <QEventLoop>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QUrl>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QProcess>
#include <QSettings>
#include <QStandardPaths>
#include <QThread>
#include <QTimer>
#include <QUrl>

namespace {
const char kAppVersion[] = "0.1.29";
const char kApiVersion[] = "1";

QString helperPath()
{
    return QDir(QCoreApplication::applicationDirPath()).filePath(QStringLiteral("bin/rayutd"));
}

QString tokenPath()
{
    const QString dir = QStandardPaths::writableLocation(QStandardPaths::AppConfigLocation);
    return QDir(dir).filePath(QStringLiteral("client-token"));
}

QString ruleTemplateLabel(const QString &id)
{
    if (id == QLatin1String("global")) {
        return QStringLiteral("全局代理");
    }
    if (id == QLatin1String("lan")) {
        return QStringLiteral("绕过局域网");
    }
    if (id == QLatin1String("lan-china")) {
        return QStringLiteral("绕过局域网和中国大陆");
    }
    return QStringLiteral("配置自带");
}
}

EditLineModel::EditLineModel(QObject *parent)
    : QAbstractListModel(parent)
{
}

int EditLineModel::rowCount(const QModelIndex &parent) const
{
    if (parent.isValid()) {
        return 0;
    }
    return m_lines.size();
}

QVariant EditLineModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid() || index.row() < 0 || index.row() >= m_lines.size()) {
        return QVariant();
    }
    if (role == LineRole || role == Qt::DisplayRole) {
        return m_lines.at(index.row());
    }
    return QVariant();
}

QHash<int, QByteArray> EditLineModel::roleNames() const
{
    return {{LineRole, "line"}};
}

void EditLineModel::setDocument(const QString &text)
{
    beginResetModel();
    m_lines = text.isEmpty() ? QStringList() : text.split(QLatin1Char('\n'), QString::KeepEmptyParts);
    endResetModel();
}

QString EditLineModel::document() const
{
    return m_lines.join(QLatin1Char('\n'));
}

void EditLineModel::setLine(int row, const QString &text)
{
    if (row < 0 || row >= m_lines.size() || m_lines.at(row) == text) {
        return;
    }
    m_lines[row] = text;
}

void EditLineModel::splitLine(int row, int cursor, int *focusRow, int *focusColumn)
{
    if (row < 0 || row >= m_lines.size()) {
        return;
    }
    const QString line = m_lines.at(row);
    const int cut = qBound(0, cursor, line.size());
    m_lines[row] = line.left(cut);
    beginInsertRows(QModelIndex(), row + 1, row + 1);
    m_lines.insert(row + 1, line.mid(cut));
    endInsertRows();
    const QModelIndex current = index(row, 0);
    emit dataChanged(current, current, {LineRole});
    if (focusRow) {
        *focusRow = row + 1;
    }
    if (focusColumn) {
        *focusColumn = 0;
    }
}

void EditLineModel::joinLine(int row, int *focusRow, int *focusColumn)
{
    if (row <= 0 || row >= m_lines.size()) {
        return;
    }
    const int column = m_lines.at(row - 1).size();
    m_lines[row - 1] += m_lines.at(row);
    beginRemoveRows(QModelIndex(), row, row);
    m_lines.removeAt(row);
    endRemoveRows();
    const QModelIndex previous = index(row - 1, 0);
    emit dataChanged(previous, previous, {LineRole});
    if (focusRow) {
        *focusRow = row - 1;
    }
    if (focusColumn) {
        *focusColumn = column;
    }
}

Controller::Controller(QObject *parent)
    : QObject(parent)
    , m_network(new QNetworkAccessManager(this))
    , m_coreBusy(false)
    , m_configState(QStringLiteral("ok"))
    , m_helperRunning(false)
    , m_tunRunning(false)
    , m_coreRunning(false)
    , m_configBroken(false)
    , m_versionMismatch(false)
    , m_killSwitch(false)
    , m_allowLan(false)
    , m_lanPort(0)
    , m_networkBlocked(false)
    , m_stateQueued(false)
    , m_editLines(new EditLineModel(this))
    , m_editFocusRow(-1)
    , m_editFocusColumn(0)
    , m_sessionUpload(0)
    , m_sessionDownload(0)
    , m_uploadRate(0)
    , m_downloadRate(0)
    , m_totalUpload(0)
    , m_totalDownload(0)
    , m_delayRunning(false)
    , m_delayDone(0)
    , m_delayTotal(0)
{
}

bool Controller::helperRunning() const
{
    return m_helperRunning;
}

bool Controller::tunRunning() const
{
    return m_tunRunning;
}

bool Controller::versionMismatch() const
{
    return m_versionMismatch;
}

bool Controller::killSwitch() const
{
    return m_killSwitch;
}

bool Controller::allowLan() const
{
    return m_allowLan;
}

int Controller::lanPort() const
{
    return m_lanPort;
}

bool Controller::networkBlocked() const
{
    return m_networkBlocked;
}

QString Controller::summary() const
{
    if (!m_helperRunning) {
        return QStringLiteral("助手未运行");
    }
    if (m_networkBlocked) {
        return QStringLiteral("网络已拦截，等待关闭或重连");
    }
    if (m_coreRunning && m_tunRunning) {
        return QStringLiteral("代理打开");
    }
    if (!m_coreRunning && !m_tunRunning) {
        if (m_configState == QLatin1String("missing") || m_configState == QLatin1String("error") || m_configBroken) {
            return QStringLiteral("配置错误");
        }
        return QStringLiteral("代理关闭");
    }
    return QStringLiteral("核心未运行");
}

QString Controller::message() const
{
    return m_message;
}

QString Controller::profileText() const
{
    return m_profileText;
}

QString Controller::ruleTemplate() const
{
    return m_ruleTemplate;
}

QVariantList Controller::proxyGroups() const
{
    return m_proxyGroups;
}

QVariantList Controller::sessionLogs() const
{
    return m_sessionLogs;
}

QVariantList Controller::sessionConnections() const
{
    return m_sessionConnections;
}

qint64 Controller::sessionUpload() const
{
    return m_sessionUpload;
}

qint64 Controller::sessionDownload() const
{
    return m_sessionDownload;
}

qint64 Controller::uploadRate() const
{
    return m_uploadRate;
}

qint64 Controller::downloadRate() const
{
    return m_downloadRate;
}

qint64 Controller::totalUpload() const
{
    return m_totalUpload;
}

qint64 Controller::totalDownload() const
{
    return m_totalDownload;
}

QVariantList Controller::trafficSamples() const
{
    return m_trafficSamples;
}

QVariantList Controller::profileGroups() const
{
    return m_profileGroups;
}

QVariantList Controller::groupNodes() const
{
    return m_groupNodes;
}

QVariantList Controller::proxySelectors() const
{
    return m_proxySelectors;
}

QString Controller::selectorName() const
{
    return m_selectorName;
}

QString Controller::viewedGroup() const
{
    return m_viewedGroup;
}

QString Controller::activeGroup() const
{
    return m_activeGroup;
}

QString Controller::groupKind() const
{
    return m_groupKind;
}

QString Controller::groupURL() const
{
    return m_groupURL;
}

bool Controller::delayRunning() const
{
    return m_delayRunning;
}

int Controller::delayDone() const
{
    return m_delayDone;
}

int Controller::delayTotal() const
{
    return m_delayTotal;
}

EditLineModel *Controller::editLines() const
{
    return m_editLines;
}

int Controller::editFocusRow() const
{
    return m_editFocusRow;
}

int Controller::editFocusColumn() const
{
    return m_editFocusColumn;
}

QVariantList Controller::editProxies() const
{
    return m_editProxies;
}

void Controller::setEditLine(int row, const QString &text)
{
    m_editLines->setLine(row, text);
}

void Controller::splitEditLine(int row, int cursor)
{
    int focusRow = -1;
    int focusColumn = 0;
    m_editLines->splitLine(row, cursor, &focusRow, &focusColumn);
    m_editFocusRow = focusRow;
    m_editFocusColumn = focusColumn;
    emit editFocusChanged();
}

void Controller::joinEditLine(int row)
{
    int focusRow = -1;
    int focusColumn = 0;
    m_editLines->joinLine(row, &focusRow, &focusColumn);
    m_editFocusRow = focusRow;
    m_editFocusColumn = focusColumn;
    emit editFocusChanged();
}

QVariantList Controller::coreReleases() const
{
    return m_coreReleases;
}

QString Controller::corePlace() const
{
    return m_corePlace;
}

QString Controller::coreTag() const
{
    return m_coreTag;
}

bool Controller::coreBusy() const
{
    return m_coreBusy;
}

QString Controller::versionText() const
{
    if (!m_helperRunning) {
        return QStringLiteral("助手 — · API — · 核心 —");
    }
    if (!m_versionMismatch) {
        return m_versionText;
    }
    return m_versionText + QStringLiteral("\n版本不一致，请重新连接");
}

void Controller::setMessage(const QString &message)
{
    if (m_message == message) {
        return;
    }
    m_message = message;
    queueStateChanged();
}

void Controller::queueStateChanged()
{
    if (m_stateQueued) {
        return;
    }
    m_stateQueued = true;
    QTimer::singleShot(0, this, [this]() {
        m_stateQueued = false;
        emit stateChanged();
    });
}

bool Controller::readToken()
{
    QFile file(tokenPath());
    if (!file.open(QIODevice::ReadOnly)) {
        m_token.clear();
        return false;
    }
    m_token = QString::fromUtf8(file.readAll()).trimmed();
    return !m_token.isEmpty();
}

void Controller::refresh()
{
    QByteArray body;
    if (!request(QStringLiteral("GET"), QStringLiteral("/v1/status"), QByteArray(), &body, 30000)) {
        const bool changed = m_helperRunning || m_tunRunning || m_coreRunning || m_versionMismatch || m_configBroken || m_killSwitch || m_allowLan || m_lanPort != 0 || m_networkBlocked || !m_profileText.isEmpty() || !m_versionText.isEmpty() || !m_ruleTemplate.isEmpty()
            || m_sessionUpload != 0 || m_sessionDownload != 0 || m_uploadRate != 0 || m_downloadRate != 0
            || m_totalUpload != 0 || m_totalDownload != 0 || !m_trafficSamples.isEmpty();
        m_helperRunning = false;
        m_tunRunning = false;
        m_coreRunning = false;
        m_configBroken = false;
        m_versionMismatch = false;
        m_killSwitch = false;
        m_allowLan = false;
        m_lanPort = 0;
        m_networkBlocked = false;
        m_configState = QStringLiteral("ok");
        m_profileText.clear();
        m_ruleTemplate.clear();
        m_versionText.clear();
        m_sessionUpload = 0;
        m_sessionDownload = 0;
        m_uploadRate = 0;
        m_downloadRate = 0;
        m_totalUpload = 0;
        m_totalDownload = 0;
        m_trafficSamples.clear();
        m_profileGroups.clear();
        m_groupNodes.clear();
        m_proxySelectors.clear();
        m_selectorName.clear();
        m_viewedGroup.clear();
        m_activeGroup.clear();
        m_groupKind.clear();
        m_groupURL.clear();
        if (changed) {
            queueStateChanged();
        }
        return;
    }
    applyStatus(body);
    refreshTraffic();
    refreshProfileGroups();
    QByteArray profile;
    if (request(QStringLiteral("GET"), QStringLiteral("/v1/profiles"), QByteArray(), &profile, 30000)) {
        applyProfile(profile);
    }
}

void Controller::startHelper(QString password)
{
    refresh();
    if (m_helperRunning) {
        password.fill(QLatin1Char(' '));
        password.clear();
        setMessage(QStringLiteral("助手已在运行"));
        return;
    }

    QByteArray secret = password.toUtf8();
    password.fill(QLatin1Char(' '));
    password.clear();

    QProcess process;
    process.start(QStringLiteral("/usr/bin/sudo"), {
        QStringLiteral("-S"),
        QStringLiteral("-p"),
        QStringLiteral(""),
        QStringLiteral("--"),
        helperPath(),
        QStringLiteral("--session"),
    });
    if (!process.waitForStarted(5000)) {
        secret.fill('\0');
        setMessage(QStringLiteral("无法启动 sudo"));
        return;
    }
    process.write(secret);
    process.write("\n");
    secret.fill('\0');
    process.closeWriteChannel();
    if (!process.waitForFinished(30000)) {
        process.kill();
        setMessage(QStringLiteral("启动超时"));
        return;
    }
    // The new helper replaces the bearer file. A token cached from the
    // previous process is rejected once, which used to look like a bad password.
    m_token.clear();
    for (int attempt = 0; attempt < 20; ++attempt) {
        readToken();
        refresh();
        if (m_helperRunning) {
            setMessage(QString());
            return;
        }
        QThread::msleep(100);
    }
    setMessage(QStringLiteral("连接失败"));
}

void Controller::enableTun()
{
    if (m_versionMismatch) {
        setMessage(QStringLiteral("版本不一致，请重新连接"));
        return;
    }
    QByteArray body;
    if (!request(QStringLiteral("POST"), QStringLiteral("/v1/tun/enable"), QByteArray(), &body, 30000)) {
        m_configBroken = true;
        queueStateChanged();
        return;
    }
    m_configBroken = false;
    applyStatus(body);
    setMessage(QString());
}

void Controller::disableTun()
{
    if (m_versionMismatch) {
        setMessage(QStringLiteral("版本不一致，请重新连接"));
        return;
    }
    QByteArray body;
    if (request(QStringLiteral("POST"), QStringLiteral("/v1/tun/disable"), QByteArray(), &body, 30000)) {
        applyStatus(body);
        setMessage(QString());
    }
}

void Controller::refreshStatus()
{
    QByteArray body;
    if (!request(QStringLiteral("GET"), QStringLiteral("/v1/status"), QByteArray(), &body, 3000)) {
        return;
    }
    applyStatus(body);
}

void Controller::setKillSwitch(bool on)
{
    QJsonObject object;
    object.insert(QStringLiteral("enabled"), on);
    QByteArray body;
    if (!request(QStringLiteral("POST"), QStringLiteral("/v1/kill-switch"), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 15000)) {
        return;
    }
    m_killSwitch = QJsonDocument::fromJson(body).object().value(QStringLiteral("enabled")).toBool();
    queueStateChanged();
}

void Controller::applyCoreCatalog(const QByteArray &body)
{
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    QVariantList list;
    const QJsonArray releases = object.value(QStringLiteral("releases")).toArray();
    for (const QJsonValue &value : releases) {
        const QJsonObject release = value.toObject();
        QVariantMap item;
        item.insert(QStringLiteral("tag"), release.value(QStringLiteral("tag")).toString());
        item.insert(QStringLiteral("license"), release.value(QStringLiteral("license")).toString());
        item.insert(QStringLiteral("source"), release.value(QStringLiteral("source")).toString());
        item.insert(QStringLiteral("installed"), release.value(QStringLiteral("installed")).toBool());
        list.append(item);
    }
    m_coreReleases = list;
    m_corePlace = object.value(QStringLiteral("place")).toString();
    m_coreTag = object.value(QStringLiteral("tag")).toString();
    queueStateChanged();
}

void Controller::refreshCores()
{
    QByteArray body;
    if (!request(QStringLiteral("GET"), QStringLiteral("/v1/cores"), QByteArray(), &body, 5000)) {
        return;
    }
    applyCoreCatalog(body);
}

void Controller::installCore(const QString &tag, bool disconnectFirst)
{
    if (m_coreBusy) {
        return;
    }
    if (m_versionMismatch) {
        setMessage(QStringLiteral("版本不一致，请重新连接"));
        return;
    }
    if (disconnectFirst && (m_tunRunning || m_coreRunning)) {
        setMessage(QStringLiteral("正在断开"));
        QByteArray body;
        if (!request(QStringLiteral("POST"), QStringLiteral("/v1/tun/disable"), QByteArray(), &body, 30000)) {
            return;
        }
        applyStatus(body);
    }
    if (m_tunRunning || m_coreRunning) {
        setMessage(QStringLiteral("请先关闭代理"));
        return;
    }
    m_coreBusy = true;
    setMessage(QStringLiteral("正在校验并安装"));
    QJsonObject object;
    object.insert(QStringLiteral("tag"), tag);
    QByteArray body;
    if (!request(QStringLiteral("POST"), QStringLiteral("/v1/cores/install"), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 180000)) {
        m_coreBusy = false;
        queueStateChanged();
        return;
    }
    m_coreInstallBody = body;
    QTimer::singleShot(0, this, [this]() {
        applyCoreCatalog(m_coreInstallBody);
        m_coreInstallBody.clear();
        m_coreBusy = false;
        refreshStatus();
        setMessage(QStringLiteral("已安装"));
    });
}

void Controller::setAllowLan(bool on)
{
    setMessage(QStringLiteral("正在应用"));
    QJsonObject object;
    object.insert(QStringLiteral("enabled"), on);
    QByteArray body;
    if (!request(QStringLiteral("POST"), QStringLiteral("/v1/lan"), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 40000)) {
        return;
    }
    const QJsonObject response = QJsonDocument::fromJson(body).object();
    m_allowLan = response.value(QStringLiteral("enabled")).toBool();
    m_lanPort = response.value(QStringLiteral("port")).toInt();
    queueStateChanged();
    refreshStatus();
    setMessage(QString());
}

void Controller::toggleProxy()
{
    if (!m_helperRunning || m_versionMismatch) {
        if (m_versionMismatch) {
            setMessage(QStringLiteral("版本不一致，请重新连接"));
        }
        return;
    }
    if (m_tunRunning) {
        disableTun();
        return;
    }
    enableTun();
}

void Controller::importContent(const QString &content)
{
    QJsonObject object;
    object.insert(QStringLiteral("name"), QStringLiteral("本地"));
    object.insert(QStringLiteral("content"), content);
    postCatalog(QStringLiteral("/v1/groups/import-content"), QJsonDocument(object).toJson(QJsonDocument::Compact), QStringLiteral("已导入"), true);
}

static int decodeGrayImage(const QImage &image, char *out, int outCap)
{
    if (image.isNull() || image.width() <= 0 || image.height() <= 0)
        return -1;
    const QImage gray = image.format() == QImage::Format_Grayscale8 ? image : image.convertToFormat(QImage::Format_Grayscale8);
    const qint64 bytes = qint64(gray.width()) * qint64(gray.height());
    if (bytes <= 0 || bytes > 8000000)
        return -1;
    QByteArray pixels(int(bytes), '\0');
    for (int y = 0; y < gray.height(); ++y)
        memcpy(pixels.data() + y * gray.width(), gray.constScanLine(y), (size_t)gray.width());
    return rayut_decode_gray(reinterpret_cast<const unsigned char *>(pixels.constData()), gray.width(), gray.height(), out, outCap);
}

static QImage fitLongest(const QImage &image, int side)
{
    const int longest = qMax(image.width(), image.height());
    if (side <= 0 || longest <= side)
        return image;
    return image.scaled(side, side, Qt::KeepAspectRatio, Qt::FastTransformation);
}

static QImage centerFraction(const QImage &image, double fraction)
{
    const int width = qBound(1, int(image.width() * fraction), image.width());
    const int height = qBound(1, int(image.height() * fraction), image.height());
    return image.copy((image.width() - width) / 2, (image.height() - height) / 2, width, height);
}

void Controller::importFromImage(const QUrl &url)
{
    const QString path = url.isLocalFile() ? url.toLocalFile() : url.toString();
    QImageReader reader(path);
    reader.setAutoTransform(true);
    const QImage image = reader.read();
    if (image.isNull() || image.width() <= 0 || image.height() <= 0) {
        setMessage(QStringLiteral("无法识别的图片"));
        return;
    }
    const QImage gray = image.convertToFormat(QImage::Format_Grayscale8);
    QByteArray text(8896, '\0');
    int length = -1;
    if (qMax(gray.width(), gray.height()) <= 2000)
        length = decodeGrayImage(gray, text.data(), text.size());
    const int sides[] = {1600, 1000, 640};
    for (int side : sides) {
        if (length > 0)
            break;
        length = decodeGrayImage(fitLongest(gray, side), text.data(), text.size());
    }
    const double crops[] = {0.7, 0.5, 0.3};
    for (double fraction : crops) {
        if (length > 0)
            break;
        length = decodeGrayImage(fitLongest(centerFraction(gray, fraction), 1400), text.data(), text.size());
    }
    if (length <= 0) {
        setMessage(QStringLiteral("无法识别的图片"));
        return;
    }
    const QString value = QString::fromUtf8(text.constData(), length).trimmed();
    if (rayut_route_text(value.toUtf8().constData()) == 1) {
        importURL(value);
        return;
    }
    importContent(value);
}

void Controller::importURL(const QString &url)
{
    QJsonObject object;
    object.insert(QStringLiteral("url"), url);
    postCatalog(QStringLiteral("/v1/groups/import-url"), QJsonDocument(object).toJson(QJsonDocument::Compact), QStringLiteral("已导入"), true);
}

QString Controller::clipboardText() const
{
    QClipboard *clipboard = QGuiApplication::clipboard();
    if (!clipboard) {
        return QString();
    }
    return clipboard->text().trimmed();
}

void Controller::importClipboard()
{
    const QString text = clipboardText();
    if (text.isEmpty()) {
        setMessage(QStringLiteral("剪贴板是空的"));
        return;
    }
    importContent(text);
}

void Controller::importClipboardScheme(const QString &scheme)
{
    const QString text = clipboardText();
    const QString lower = text.toLower();
    bool matches = lower.startsWith(scheme.toLower() + QStringLiteral("://"));
    if (scheme == QLatin1String("hysteria2") && lower.startsWith(QStringLiteral("hy2://"))) {
        matches = true;
    }
    if (!matches) {
        setMessage(QStringLiteral("请先复制一条该类型的链接"));
        return;
    }
    importContent(text);
}

QString Controller::groupPath(const QString &id, const QString &action) const
{
    QString path = QStringLiteral("/v1/groups/") + QString::fromUtf8(QUrl::toPercentEncoding(id));
    if (!action.isEmpty()) {
        path += QLatin1Char('/') + action;
    }
    return path;
}

void Controller::refreshProfileGroups()
{
    QByteArray body;
    if (!request(QStringLiteral("GET"), QStringLiteral("/v1/groups"), QByteArray(), &body, 30000)) {
        if (!m_helperRunning) {
            m_profileGroups.clear();
            m_groupNodes.clear();
            m_proxySelectors.clear();
            m_selectorName.clear();
            m_viewedGroup.clear();
            m_activeGroup.clear();
            m_groupKind.clear();
            queueStateChanged();
        }
        return;
    }
    applyCatalog(body, true);
}

void Controller::showGroup(const QString &id)
{
    if (id.isEmpty() || id == m_viewedGroup) {
        return;
    }
    m_viewedGroup = id;
    for (const QVariant &value : m_profileGroups) {
        const QVariantMap group = value.toMap();
        if (group.value(QStringLiteral("id")).toString() != id) {
            continue;
        }
        m_groupKind = group.value(QStringLiteral("kind")).toString();
        const QString templ = group.value(QStringLiteral("template")).toString();
        if (m_ruleTemplate != templ) {
            m_ruleTemplate = templ;
        }
        break;
    }
    queueStateChanged();
    loadNodes(id);
}

void Controller::useGroup(const QString &id)
{
    if (m_tunRunning) {
        setMessage(QStringLiteral("请先关闭代理"));
        return;
    }
    postCatalog(groupPath(id, QStringLiteral("use")), QByteArrayLiteral("{}"), QStringLiteral("已切换"), true);
}

void Controller::refreshGroup(const QString &id)
{
    postCatalog(groupPath(id, QStringLiteral("refresh")), QByteArrayLiteral("{}"), QStringLiteral("已刷新"), true);
}

void Controller::deleteGroup(const QString &id)
{
    postCatalog(groupPath(id, QStringLiteral("delete")), QByteArrayLiteral("{}"), QStringLiteral("已删除"), true);
}

void Controller::clearGroup(const QString &id)
{
    postNodes(groupPath(id, QStringLiteral("clear")), QByteArrayLiteral("{}"), QStringLiteral("已删除节点"));
}

void Controller::deleteNode(const QString &id, int index)
{
    QJsonObject object;
    object.insert(QStringLiteral("index"), index);
    postNodes(groupPath(id, QStringLiteral("node-delete")), QJsonDocument(object).toJson(QJsonDocument::Compact), QStringLiteral("已删除"));
}

void Controller::showSelector(const QString &name)
{
    if (name.isEmpty() || name == m_selectorName) {
        return;
    }
    m_selectorName = name;
    QSettings settings(QSettings::IniFormat, QSettings::UserScope, QStringLiteral("rayut.rayut"), QStringLiteral("rayut"));
    settings.setValue(QStringLiteral("policy-group"), name);
    queueStateChanged();
}

void Controller::selectNode(const QString &id, const QString &group, const QString &name)
{
    QJsonObject object;
    object.insert(QStringLiteral("group"), group);
    object.insert(QStringLiteral("name"), name);
    QByteArray body;
    if (!request(QStringLiteral("POST"), groupPath(id, QStringLiteral("select")), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 20000)) {
        return;
    }
    loadSelectors(id);
    setMessage(QStringLiteral("已选择"));
}

void Controller::testNode(const QString &id, const QString &name)
{
    QJsonObject object;
    object.insert(QStringLiteral("name"), name);
    QByteArray body;
    if (!request(QStringLiteral("POST"), groupPath(id, QStringLiteral("delay")), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 15000)) {
        return;
    }
    const QJsonObject response = QJsonDocument::fromJson(body).object();
    m_delayRunning = response.value(QStringLiteral("running")).toBool();
    m_delayDone = response.value(QStringLiteral("done")).toInt();
    m_delayTotal = response.value(QStringLiteral("total")).toInt();
    queueStateChanged();
}

void Controller::testGroup(const QString &id)
{
    QByteArray body;
    if (!request(QStringLiteral("POST"), groupPath(id, QStringLiteral("delay-all")), QByteArrayLiteral("{}"), &body, 15000)) {
        return;
    }
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    m_delayRunning = object.value(QStringLiteral("running")).toBool();
    m_delayDone = object.value(QStringLiteral("done")).toInt();
    m_delayTotal = object.value(QStringLiteral("total")).toInt();
    queueStateChanged();
}

void Controller::pollGroupDelay(const QString &id)
{
    if (id.isEmpty()) {
        return;
    }
    QByteArray body;
    if (!request(QStringLiteral("GET"), groupPath(id, QStringLiteral("delay-all")), QByteArray(), &body, 3000)) {
        return;
    }
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    const bool running = object.value(QStringLiteral("running")).toBool();
    const int done = object.value(QStringLiteral("done")).toInt();
    const int total = object.value(QStringLiteral("total")).toInt();
    const bool changed = running != m_delayRunning || done != m_delayDone || total != m_delayTotal;
    m_delayRunning = running;
    m_delayDone = done;
    m_delayTotal = total;
    if (!running) {
        loadNodes(id);
    } else if (changed) {
        queueStateChanged();
    }
}

void Controller::copyExport(const QByteArray &body)
{
    const QString text = QJsonDocument::fromJson(body).object().value(QStringLiteral("text")).toString();
    if (text.trimmed().isEmpty()) {
        setMessage(QStringLiteral("没有可导出的分享链接"));
        return;
    }
    QClipboard *clipboard = QGuiApplication::clipboard();
    if (!clipboard) {
        setMessage(QStringLiteral("请求失败"));
        return;
    }
    clipboard->setText(text);
    setMessage(QStringLiteral("已复制"));
}

void Controller::exportGroup(const QString &id)
{
    QByteArray body;
    if (!request(QStringLiteral("POST"), groupPath(id, QStringLiteral("export")), QByteArrayLiteral("{}"), &body, 15000)) {
        return;
    }
    copyExport(body);
}

QString Controller::shareImage() const
{
    return m_shareImage;
}

QString Controller::shareNote() const
{
    return m_shareNote;
}

void Controller::showShareNote(const QString &note)
{
    m_shareImage.clear();
    m_shareNote = note;
    setMessage(QString());
    emit shareChanged();
}

void Controller::clearShare()
{
    if (m_shareImage.isEmpty() && m_shareNote.isEmpty()) {
        return;
    }
    m_shareImage.clear();
    m_shareNote.clear();
    emit shareChanged();
}

bool Controller::exportNode(const QString &id, const QString &name)
{
    QJsonObject object;
    object.insert(QStringLiteral("name"), name);
    QByteArray body;
    if (!request(QStringLiteral("POST"), groupPath(id, QStringLiteral("export")), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 15000)) {
        if (QString::fromUtf8(body).trimmed() == QLatin1String("not found")) {
            showShareNote(QStringLiteral("没有存下来的分享链接"));
            return true;
        }
        clearShare();
        return false;
    }
    const QString text = QJsonDocument::fromJson(body).object().value(QStringLiteral("text")).toString().trimmed();
    body.fill('\0');
    if (text.isEmpty()) {
        showShareNote(QStringLiteral("没有存下来的分享链接"));
        return true;
    }
    unsigned char *pixels = nullptr;
    const int side = rayut_encode_qr(text.toUtf8().constData(), &pixels, 8);
    if (side < 1 || !pixels) {
        free(pixels);
        showShareNote(QStringLiteral("这条链接画不成码"));
        return true;
    }
    const QImage view(pixels, side, side, side, QImage::Format_Grayscale8);
    const QImage owned = view.copy();
    free(pixels);
    QByteArray png;
    QBuffer buffer(&png);
    if (!buffer.open(QIODevice::WriteOnly) || !owned.save(&buffer, "PNG")) {
        showShareNote(QStringLiteral("这条链接画不成码"));
        return true;
    }
    QClipboard *clipboard = QGuiApplication::clipboard();
    if (clipboard) {
        clipboard->setText(text);
        setMessage(QStringLiteral("已复制"));
    } else {
        setMessage(QString());
    }
    m_shareNote.clear();
    m_shareImage = QStringLiteral("data:image/png;base64,") + QString::fromLatin1(png.toBase64());
    emit shareChanged();
    return true;
}

void Controller::saveGroup(const QString &id, const QString &name, const QString &url)
{
    QJsonObject object;
    object.insert(QStringLiteral("name"), name);
    if (!url.trimmed().isEmpty()) {
        object.insert(QStringLiteral("url"), url);
    }
    postCatalog(groupPath(id, QString()), QJsonDocument(object).toJson(QJsonDocument::Compact), QStringLiteral("已保存"), false);
    m_groupURL = url;
}

void Controller::loadGroupDetail(const QString &id)
{
    QByteArray body;
    if (!request(QStringLiteral("GET"), groupPath(id, QString()), QByteArray(), &body, 15000)) {
        m_groupURL.clear();
        setMessage(messageFor(QString::fromUtf8(body).trimmed()));
        return;
    }
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    m_groupURL = object.value(QStringLiteral("url")).toString();
    const QJsonObject group = object.value(QStringLiteral("group")).toObject();
    if (!group.isEmpty()) {
        m_groupKind = group.value(QStringLiteral("kind")).toString();
        m_ruleTemplate = group.value(QStringLiteral("template")).toString();
    }
    queueStateChanged();
}

void Controller::restartProxy()
{
    if (m_versionMismatch) {
        setMessage(QStringLiteral("版本不一致，请重新连接"));
        return;
    }
    if (m_tunRunning) {
        disableTun();
        if (m_tunRunning) {
            return;
        }
    }
    enableTun();
}

void Controller::loadNodes(const QString &id)
{
    if (id.isEmpty()) {
        m_groupNodes.clear();
        m_proxySelectors.clear();
        m_selectorName.clear();
        queueStateChanged();
        return;
    }
    QByteArray body;
    if (!request(QStringLiteral("GET"), groupPath(id, QStringLiteral("nodes")), QByteArray(), &body, 30000)) {
        m_groupNodes.clear();
        m_proxySelectors.clear();
        queueStateChanged();
        return;
    }
    applyNodes(body);
    loadSelectors(id);
}

void Controller::loadSelectors(const QString &id)
{
    if (id.isEmpty()) {
        m_proxySelectors.clear();
        m_selectorName.clear();
        queueStateChanged();
        return;
    }
    QByteArray body;
    if (!request(QStringLiteral("GET"), groupPath(id, QStringLiteral("selectors")), QByteArray(), &body, 30000)) {
        m_proxySelectors.clear();
        queueStateChanged();
        return;
    }
    applySelectors(body);
}

void Controller::applyCatalog(const QByteArray &body, bool reloadNodes)
{
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    const QJsonArray groups = object.value(QStringLiteral("groups")).toArray();
    QVariantList list;
    QString active;
    bool viewedHere = false;
    QHash<QString, int> previousCounts;
    for (const QVariant &value : m_profileGroups) {
        const QVariantMap group = value.toMap();
        previousCounts.insert(group.value(QStringLiteral("id")).toString(), group.value(QStringLiteral("count")).toInt());
    }
    QString added;
    QString grown;
    int grownCount = 0;
    for (const QJsonValue &value : groups) {
        const QJsonObject group = value.toObject();
        QVariantMap row;
        const QString id = group.value(QStringLiteral("id")).toString();
        const int count = group.value(QStringLiteral("count")).toInt();
        row.insert(QStringLiteral("id"), id);
        row.insert(QStringLiteral("name"), group.value(QStringLiteral("name")).toString());
        row.insert(QStringLiteral("kind"), group.value(QStringLiteral("kind")).toString());
        row.insert(QStringLiteral("count"), count);
        row.insert(QStringLiteral("active"), group.value(QStringLiteral("active")).toBool());
        row.insert(QStringLiteral("template"), group.value(QStringLiteral("template")).toString());
        list.append(row);
        if (group.value(QStringLiteral("active")).toBool()) {
            active = id;
        }
        if (id == m_viewedGroup) {
            viewedHere = true;
        }
        if (!previousCounts.isEmpty() && !previousCounts.contains(id)) {
            added = id;
        }
        if (previousCounts.contains(id) && count > previousCounts.value(id)) {
            grown = id;
            grownCount++;
        }
    }
    if (!object.value(QStringLiteral("active")).toString().isEmpty()) {
        active = object.value(QStringLiteral("active")).toString();
    }
    const QString oldViewed = m_viewedGroup;
    m_profileGroups = list;
    m_activeGroup = active;
    if (!added.isEmpty()) {
        m_viewedGroup = added;
        viewedHere = true;
    } else if (grownCount == 1) {
        m_viewedGroup = grown;
        viewedHere = true;
    }
    if (!viewedHere) {
        m_viewedGroup = !active.isEmpty() ? active : (list.isEmpty() ? QString() : list.first().toMap().value(QStringLiteral("id")).toString());
    }
    m_groupKind.clear();
    for (const QVariant &value : m_profileGroups) {
        const QVariantMap group = value.toMap();
        if (group.value(QStringLiteral("id")).toString() != m_viewedGroup) {
            continue;
        }
        m_groupKind = group.value(QStringLiteral("kind")).toString();
        m_ruleTemplate = group.value(QStringLiteral("template")).toString();
        break;
    }
    queueStateChanged();
    if (reloadNodes || oldViewed != m_viewedGroup || !added.isEmpty()) {
        loadNodes(m_viewedGroup);
    }
}

void Controller::applyNodes(const QByteArray &body)
{
    const QJsonArray nodes = QJsonDocument::fromJson(body).object().value(QStringLiteral("nodes")).toArray();
    QVariantList list;
    for (const QJsonValue &value : nodes) {
        const QJsonObject node = value.toObject();
        QVariantMap row;
        row.insert(QStringLiteral("index"), node.value(QStringLiteral("index")).toInt());
        row.insert(QStringLiteral("name"), node.value(QStringLiteral("name")).toString());
        row.insert(QStringLiteral("type"), node.value(QStringLiteral("type")).toString());
        row.insert(QStringLiteral("network"), node.value(QStringLiteral("network")).toString());
        row.insert(QStringLiteral("delay"), node.value(QStringLiteral("delay")).toInt());
        row.insert(QStringLiteral("selected"), node.value(QStringLiteral("selected")).toBool());
        row.insert(QStringLiteral("shareable"), node.value(QStringLiteral("shareable")).toBool());
        list.append(row);
    }
    m_groupNodes = list;
    queueStateChanged();
}

void Controller::applySelectors(const QByteArray &body)
{
    const QJsonArray groups = QJsonDocument::fromJson(body).object().value(QStringLiteral("groups")).toArray();
    QVariantList list;
    bool keep = false;
    QString firstManual;
    QString firstName;
    QString trafficName;
    QSettings settings(QSettings::IniFormat, QSettings::UserScope, QStringLiteral("rayut.rayut"), QStringLiteral("rayut"));
    const QString saved = settings.value(QStringLiteral("policy-group")).toString();
    bool savedHere = false;
    for (const QJsonValue &value : groups) {
        const QJsonObject group = value.toObject();
        const QString name = group.value(QStringLiteral("name")).toString();
        QVariantMap row;
        row.insert(QStringLiteral("name"), name);
        row.insert(QStringLiteral("type"), group.value(QStringLiteral("type")).toString());
        row.insert(QStringLiteral("selectable"), group.value(QStringLiteral("selectable")).toBool());
        row.insert(QStringLiteral("now"), group.value(QStringLiteral("now")).toString());
        QVariantList nodes;
        for (const QJsonValue &nodeValue : group.value(QStringLiteral("nodes")).toArray()) {
            const QJsonObject node = nodeValue.toObject();
            QVariantMap item;
            item.insert(QStringLiteral("index"), node.value(QStringLiteral("index")).toInt());
            item.insert(QStringLiteral("name"), node.value(QStringLiteral("name")).toString());
            item.insert(QStringLiteral("type"), node.value(QStringLiteral("type")).toString());
            item.insert(QStringLiteral("network"), node.value(QStringLiteral("network")).toString());
            item.insert(QStringLiteral("delay"), node.value(QStringLiteral("delay")).toInt());
            item.insert(QStringLiteral("selected"), node.value(QStringLiteral("selected")).toBool());
            item.insert(QStringLiteral("shareable"), node.value(QStringLiteral("shareable")).toBool());
            nodes.append(item);
        }
        row.insert(QStringLiteral("nodes"), nodes);
        list.append(row);
        if (firstName.isEmpty()) {
            firstName = name;
        }
        if (firstManual.isEmpty() && group.value(QStringLiteral("selectable")).toBool()) {
            firstManual = name;
        }
        if (trafficName.isEmpty() && group.value(QStringLiteral("traffic")).toBool()) {
            trafficName = name;
        }
        if (!saved.isEmpty() && name == saved) {
            savedHere = true;
        }
        if (name == m_selectorName) {
            keep = true;
        }
    }
    m_proxySelectors = list;
    if (!keep) {
        if (savedHere) {
            m_selectorName = saved;
        } else if (!trafficName.isEmpty()) {
            m_selectorName = trafficName;
        } else {
            m_selectorName = !firstManual.isEmpty() ? firstManual : firstName;
        }
    }
    queueStateChanged();
}

bool Controller::postCatalog(const QString &path, const QByteArray &payload, const QString &success, bool reloadNodes)
{
    QByteArray body;
    if (!request(QStringLiteral("POST"), path, payload, &body, 60000)) {
        return false;
    }
    applyCatalog(body, reloadNodes);
    setMessage(success);
    return true;
}

bool Controller::postNodes(const QString &path, const QByteArray &payload, const QString &success)
{
    QByteArray body;
    if (!request(QStringLiteral("POST"), path, payload, &body, 60000)) {
        return false;
    }
    applyNodes(body);
    loadSelectors(m_viewedGroup);
    QByteArray groups;
    if (request(QStringLiteral("GET"), QStringLiteral("/v1/groups"), QByteArray(), &groups, 30000)) {
        applyCatalog(groups, false);
    }
    setMessage(success);
    return true;
}

void Controller::activateProfile()
{
    postProfile(QStringLiteral("/v1/profiles/activate"), QByteArray(), QStringLiteral("已激活"));
}

void Controller::refreshProfile()
{
    postProfile(QStringLiteral("/v1/profiles/refresh"), QByteArray(), QStringLiteral("已校验，当前配置未替换"));
}

void Controller::applyRuleTemplate(const QString &id)
{
    if (m_viewedGroup.isEmpty()) {
        setMessage(QStringLiteral("没有这个分组"));
        return;
    }
    if (m_viewedGroup == m_activeGroup && m_tunRunning) {
        setMessage(QStringLiteral("请先关闭代理"));
        return;
    }
    QJsonObject object;
    object.insert(QStringLiteral("id"), id);
    postCatalog(groupPath(m_viewedGroup, QStringLiteral("template")), QJsonDocument(object).toJson(QJsonDocument::Compact), QStringLiteral("已切换规则"), false);
}

void Controller::loadProfileDocument()
{
    loadGroupDocument(m_viewedGroup);
}

void Controller::loadGroupDocument(const QString &id)
{
    m_editGroup = id;
    if (id.isEmpty()) {
        m_editLines->setDocument(QString());
        m_editProxies.clear();
        queueStateChanged();
        setMessage(QStringLiteral("没有这个分组"));
        return;
    }
    QByteArray body;
    if (!request(QStringLiteral("GET"), groupPath(id, QStringLiteral("document")), QByteArray(), &body, 30000)) {
        m_editLines->setDocument(QString());
        m_editProxies.clear();
        queueStateChanged();
        setMessage(messageFor(QString::fromUtf8(body).trimmed()));
        return;
    }
    applyEditDocument(body);
    setMessage(QStringLiteral("已载入当前配置"));
}

bool Controller::previewProfile()
{
    QJsonObject object;
    object.insert(QStringLiteral("text"), m_editLines->document());
    QByteArray body;
    if (!request(QStringLiteral("POST"), QStringLiteral("/v1/profiles/preview"), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 30000)) {
        return false;
    }
    applyEditDocument(body);
    return true;
}

bool Controller::applyProxyEdit(int index, const QString &name, const QString &type, const QString &server, int port, const QString &network, bool tls, bool udp, const QString &secret)
{
    QJsonObject edit;
    edit.insert(QStringLiteral("index"), index);
    edit.insert(QStringLiteral("name"), name);
    edit.insert(QStringLiteral("type"), type);
    edit.insert(QStringLiteral("server"), server);
    edit.insert(QStringLiteral("port"), port);
    edit.insert(QStringLiteral("network"), network);
    edit.insert(QStringLiteral("tls"), tls);
    edit.insert(QStringLiteral("udp"), udp);
    edit.insert(QStringLiteral("secret"), secret);
    QJsonArray edits;
    edits.append(edit);
    QJsonObject object;
    object.insert(QStringLiteral("text"), m_editLines->document());
    object.insert(QStringLiteral("edits"), edits);
    QByteArray body;
    if (!request(QStringLiteral("POST"), QStringLiteral("/v1/profiles/preview"), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 30000)) {
        return false;
    }
    applyEditDocument(body);
    setMessage(QStringLiteral("已写回文本，尚未保存"));
    return true;
}

void Controller::saveProfileText()
{
    if (m_editGroup.isEmpty()) {
        setMessage(QStringLiteral("没有这个分组"));
        return;
    }
    if (m_editGroup == m_activeGroup && m_tunRunning) {
        setMessage(QStringLiteral("请先关闭代理"));
        return;
    }
    setMessage(QStringLiteral("正在校验…"));
    QJsonObject object;
    object.insert(QStringLiteral("text"), m_editLines->document());
    QByteArray body;
    if (!request(QStringLiteral("POST"), groupPath(m_editGroup, QStringLiteral("edit")), QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 60000)) {
        return;
    }
    applyEditDocument(body);
    QByteArray groups;
    if (request(QStringLiteral("GET"), QStringLiteral("/v1/groups"), QByteArray(), &groups, 30000)) {
        applyCatalog(groups, true);
    }
    setMessage(QStringLiteral("已保存"));
}

void Controller::applyEditDocument(const QByteArray &body)
{
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    const QJsonArray proxies = object.value(QStringLiteral("proxies")).toArray();
    QVariantList rows;
    for (const QJsonValue &value : proxies) {
        const QJsonObject proxy = value.toObject();
        QVariantMap row;
        row.insert(QStringLiteral("index"), proxy.value(QStringLiteral("index")).toInt());
        row.insert(QStringLiteral("name"), proxy.value(QStringLiteral("name")).toString());
        row.insert(QStringLiteral("type"), proxy.value(QStringLiteral("type")).toString());
        row.insert(QStringLiteral("server"), proxy.value(QStringLiteral("server")).toString());
        row.insert(QStringLiteral("port"), proxy.value(QStringLiteral("port")).toInt());
        row.insert(QStringLiteral("network"), proxy.value(QStringLiteral("network")).toString());
        row.insert(QStringLiteral("tls"), proxy.value(QStringLiteral("tls")).toBool());
        row.insert(QStringLiteral("udp"), proxy.value(QStringLiteral("udp")).toBool());
        row.insert(QStringLiteral("hasSecret"), proxy.value(QStringLiteral("hasSecret")).toBool());
        rows.append(row);
    }
    m_editLines->setDocument(object.value(QStringLiteral("text")).toString());
    m_editFocusRow = -1;
    m_editProxies = rows;
    queueStateChanged();
}

void Controller::refreshGroups()
{
    QByteArray body;
    if (!request(QStringLiteral("GET"), QStringLiteral("/v1/proxy-groups"), QByteArray(), &body, 15000)) {
        m_proxyGroups.clear();
        setMessage(messageFor(QString::fromUtf8(body).trimmed()));
        queueStateChanged();
        return;
    }
    applyGroups(body);
}

void Controller::selectProxy(const QString &group, const QString &name)
{
    QJsonObject object;
    object.insert(QStringLiteral("name"), name);
    const QString path = QStringLiteral("/v1/proxy-groups/")
        + QString::fromUtf8(QUrl::toPercentEncoding(group))
        + QStringLiteral("/selection");
    QByteArray body;
    if (!request(QStringLiteral("PUT"), path, QJsonDocument(object).toJson(QJsonDocument::Compact), &body, 15000)) {
        return;
    }
    applyGroups(body);
    setMessage(QStringLiteral("已选择"));
}

void Controller::refreshTraffic()
{
    QByteArray body;
    if (!request(QStringLiteral("GET"), QStringLiteral("/v1/traffic"), QByteArray(), &body, 3000)) {
        const bool clearTotals = !m_helperRunning;
        const bool changed = m_sessionUpload != 0 || m_sessionDownload != 0 || m_uploadRate != 0 || m_downloadRate != 0 || !m_trafficSamples.isEmpty()
            || (clearTotals && (m_totalUpload != 0 || m_totalDownload != 0));
        m_sessionUpload = 0;
        m_sessionDownload = 0;
        m_uploadRate = 0;
        m_downloadRate = 0;
        m_trafficSamples.clear();
        if (clearTotals) {
            m_totalUpload = 0;
            m_totalDownload = 0;
        }
        if (changed) {
            queueStateChanged();
        }
        return;
    }
    applyTraffic(body);
}

void Controller::refreshSession()
{
    QByteArray logs;
    if (!request(QStringLiteral("GET"), QStringLiteral("/v1/logs"), QByteArray(), &logs, 15000)) {
        m_sessionLogs.clear();
        m_sessionConnections.clear();
        setMessage(messageFor(QString::fromUtf8(logs).trimmed()));
        queueStateChanged();
        return;
    }
    applyLogs(logs);
    QByteArray connections;
    if (!request(QStringLiteral("GET"), QStringLiteral("/v1/connections"), QByteArray(), &connections, 15000)) {
        m_sessionConnections.clear();
        setMessage(messageFor(QString::fromUtf8(connections).trimmed()));
        queueStateChanged();
        return;
    }
    applyConnections(connections);
    setMessage(QString());
}

void Controller::testDelay(const QString &name)
{
    setMessage(QStringLiteral("正在测试延迟"));
    const QString path = QStringLiteral("/v1/proxies/")
        + QString::fromUtf8(QUrl::toPercentEncoding(name))
        + QStringLiteral("/delay");
    QByteArray body;
    if (!request(QStringLiteral("POST"), path, QByteArray(), &body, 20000)) {
        return;
    }
    const int delay = QJsonDocument::fromJson(body).object().value(QStringLiteral("delay")).toInt();
    setMessage(QStringLiteral("延迟 %1 ms").arg(delay));
    refreshGroups();
}

void Controller::postProfile(const QString &path, const QByteArray &payload, const QString &success)
{
    QByteArray body;
    const bool ok = request(QStringLiteral("POST"), path, payload, &body, 60000);
    if (!ok) {
        if (request(QStringLiteral("GET"), QStringLiteral("/v1/profiles"), QByteArray(), &body, 30000)) {
            applyProfile(body);
        }
        return;
    }
    applyProfile(body);
    setMessage(success);
}

bool Controller::request(const QString &method, const QString &path, const QByteArray &payload, QByteArray *response, int timeoutMs, bool allowTokenRefresh)
{
    if (m_token.isEmpty() && !readToken()) {
        return false;
    }

    QNetworkRequest networkRequest(QUrl(QStringLiteral("http://127.0.0.1:18771") + path));
    networkRequest.setRawHeader("Authorization", QByteArray("Bearer ") + m_token.toUtf8());
    if (!payload.isEmpty()) {
        networkRequest.setHeader(QNetworkRequest::ContentTypeHeader, QStringLiteral("application/json"));
    }
    QNetworkReply *reply = nullptr;
    if (method == QLatin1String("GET")) {
        reply = m_network->get(networkRequest);
    } else {
        reply = m_network->sendCustomRequest(networkRequest, method.toUtf8(), payload);
    }

    QEventLoop loop;
    QTimer timer;
    timer.setSingleShot(true);
    QObject::connect(&timer, &QTimer::timeout, &loop, &QEventLoop::quit);
    QObject::connect(reply, &QNetworkReply::finished, &loop, &QEventLoop::quit);
    timer.start(timeoutMs);
    loop.exec(QEventLoop::ExcludeUserInputEvents);
    timer.stop();

    const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
    const QByteArray body = reply->readAll();
    if (response) {
        *response = body;
    }
    if (!reply->isFinished() || reply->error() != QNetworkReply::NoError || status < 200 || status >= 300) {
        reply->deleteLater();
        if (status == 401) {
            m_token.clear();
            if (allowTokenRefresh && readToken()) {
                return this->request(method, path, payload, response, timeoutMs, false);
            }
        }
        if (method != QLatin1String("GET")) {
            setMessage(messageFor(QString::fromUtf8(body).trimmed()));
        }
        return false;
    }
    reply->deleteLater();
    return true;
}

QString Controller::messageFor(const QString &code) const
{
    if (code == QLatin1String("profile missing")) {
        return QStringLiteral("缺少 profile");
    }
    if (code == QLatin1String("core missing")) {
        return QStringLiteral("缺少核心");
    }
    if (code == QLatin1String("empty")) {
        return QStringLiteral("内容为空");
    }
    if (code == QLatin1String("too large")) {
        return QStringLiteral("内容过大");
    }
    if (code == QLatin1String("invalid yaml") || code == QLatin1String("invalid config")) {
        return QStringLiteral("配置校验失败");
    }
    if (code == QLatin1String("invalid link")) {
        return QStringLiteral("链接不完整");
    }
    if (code == QLatin1String("unrecognized link")) {
        return QStringLiteral("无法识别的链接");
    }
    if (code == QLatin1String("file scheme")) {
        return QStringLiteral("不允许 file 地址");
    }
    if (code == QLatin1String("hook")) {
        return QStringLiteral("不允许外部程序");
    }
    if (code == QLatin1String("allow-lan")) {
        return QStringLiteral("不允许打开局域网");
    }
    if (code == QLatin1String("external-controller") || code == QLatin1String("bind-address") || code == QLatin1String("secret")) {
        return QStringLiteral("控制端口超出本机");
    }
    if (code == QLatin1String("fetch failed") || code == QLatin1String("redirect")) {
        return QStringLiteral("订阅下载失败");
    }
    if (code == QLatin1String("no subscription")) {
        return QStringLiteral("这不是订阅");
    }
    if (code == QLatin1String("group missing")) {
        return QStringLiteral("没有这个分组");
    }
    if (code == QLatin1String("tun running")) {
        return QStringLiteral("请先关闭代理");
    }
    if (code == QLatin1String("disconnect first")) {
        return QStringLiteral("请先断开代理");
    }
    if (code == QLatin1String("unknown release")) {
        return QStringLiteral("清单里没有这个版本");
    }
    if (code == QLatin1String("hash mismatch")) {
        return QStringLiteral("校验失败，旧核心未替换");
    }
    if (code == QLatin1String("license mismatch")) {
        return QStringLiteral("许可证校验失败，旧核心未替换");
    }
    if (code == QLatin1String("download failed")) {
        return QStringLiteral("核心下载失败");
    }
    if (code == QLatin1String("invalid file")) {
        return QStringLiteral("文件无效，旧核心未替换");
    }
    if (code == QLatin1String("tun held") || code == QLatin1String("already-running")) {
        return QStringLiteral("旧核心仍占用网络，已拒绝再启动一份");
    }
    if (code == QLatin1String("no candidate")) {
        return QStringLiteral("没有可激活的配置");
    }
    if (code == QLatin1String("unknown template")) {
        return QStringLiteral("没有这套规则");
    }
    if (code == QLatin1String("bad field")) {
        return QStringLiteral("字段无效");
    }
    if (code == QLatin1String("core not running")) {
        return QStringLiteral("核心未运行");
    }
    if (code == QLatin1String("controller unavailable")) {
        return QStringLiteral("请先关闭再打开代理");
    }
    if (code == QLatin1String("not found")) {
        return QStringLiteral("没有这个节点");
    }
    if (code == QLatin1String("not selectable")) {
        return QStringLiteral("这个组不能手动选择");
    }
    if (code == QLatin1String("invalid name")) {
        return QStringLiteral("名称无效");
    }
    if (code == QLatin1String("selection rejected")) {
        return QStringLiteral("选择失败");
    }
    if (code == QLatin1String("delay failed")) {
        return QStringLiteral("延迟测试失败");
    }
    if (code == QLatin1String("timeout")) {
        return QStringLiteral("延迟测试超时");
    }
    return QStringLiteral("请求失败");
}

void Controller::applyGroups(const QByteArray &body)
{
    const QJsonArray groups = QJsonDocument::fromJson(body).object().value(QStringLiteral("groups")).toArray();
    QVariantList list;
    for (const QJsonValue &value : groups) {
        const QJsonObject group = value.toObject();
        const QString now = group.value(QStringLiteral("now")).toString();
        QVariantMap item;
        item.insert(QStringLiteral("name"), group.value(QStringLiteral("name")).toString());
        item.insert(QStringLiteral("now"), now);
        item.insert(QStringLiteral("selectable"), group.value(QStringLiteral("selectable")).toBool());
        QVariantList nodes;
        for (const QJsonValue &nodeValue : group.value(QStringLiteral("nodes")).toArray()) {
            const QJsonObject node = nodeValue.toObject();
            const QString name = node.value(QStringLiteral("name")).toString();
            const int delay = node.value(QStringLiteral("delay")).toInt();
            QVariantMap row;
            row.insert(QStringLiteral("name"), name);
            row.insert(QStringLiteral("delayText"), delay > 0 ? QString::number(delay) + QStringLiteral(" ms") : QStringLiteral("未测"));
            row.insert(QStringLiteral("selected"), name == now);
            nodes.append(row);
        }
        item.insert(QStringLiteral("nodes"), nodes);
        list.append(item);
    }
    std::sort(list.begin(), list.end(), [](const QVariant &left, const QVariant &right) {
        return left.toMap().value(QStringLiteral("name")).toString()
            < right.toMap().value(QStringLiteral("name")).toString();
    });
    m_proxyGroups = list;
    queueStateChanged();
}

void Controller::applyLogs(const QByteArray &body)
{
    const QJsonArray lines = QJsonDocument::fromJson(body).object().value(QStringLiteral("lines")).toArray();
    QVariantList list;
    for (const QJsonValue &value : lines) {
        const QJsonObject line = value.toObject();
        QVariantMap row;
        row.insert(QStringLiteral("type"), line.value(QStringLiteral("type")).toString());
        row.insert(QStringLiteral("payload"), line.value(QStringLiteral("payload")).toString());
        list.append(row);
    }
    m_sessionLogs = list;
    queueStateChanged();
}

void Controller::applyConnections(const QByteArray &body)
{
    const QJsonArray rows = QJsonDocument::fromJson(body).object().value(QStringLiteral("connections")).toArray();
    QVariantList list;
    for (const QJsonValue &value : rows) {
        const QJsonObject row = value.toObject();
        QVariantMap item;
        item.insert(QStringLiteral("destination"), row.value(QStringLiteral("destination")).toString());
        item.insert(QStringLiteral("rule"), row.value(QStringLiteral("rule")).toString());
        item.insert(QStringLiteral("chain"), row.value(QStringLiteral("chain")).toString());
        item.insert(QStringLiteral("upload"), QString::number(row.value(QStringLiteral("upload")).toVariant().toLongLong()));
        item.insert(QStringLiteral("download"), QString::number(row.value(QStringLiteral("download")).toVariant().toLongLong()));
        list.append(item);
    }
    m_sessionConnections = list;
    queueStateChanged();
}

void Controller::applyTraffic(const QByteArray &body)
{
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    const qint64 sessionUpload = object.value(QStringLiteral("uploadTotal")).toVariant().toLongLong();
    const qint64 sessionDownload = object.value(QStringLiteral("downloadTotal")).toVariant().toLongLong();
    const qint64 uploadRate = object.value(QStringLiteral("up")).toVariant().toLongLong();
    const qint64 downloadRate = object.value(QStringLiteral("down")).toVariant().toLongLong();
    const qint64 totalUpload = object.value(QStringLiteral("cumulativeUpload")).toVariant().toLongLong();
    const qint64 totalDownload = object.value(QStringLiteral("cumulativeDownload")).toVariant().toLongLong();
    QVariantList samples;
    const QJsonArray rows = object.value(QStringLiteral("samples")).toArray();
    for (const QJsonValue &value : rows) {
        const QJsonObject row = value.toObject();
        QVariantMap item;
        item.insert(QStringLiteral("up"), row.value(QStringLiteral("up")).toVariant().toLongLong());
        item.insert(QStringLiteral("down"), row.value(QStringLiteral("down")).toVariant().toLongLong());
        samples.append(item);
    }
    if (sessionUpload == m_sessionUpload && sessionDownload == m_sessionDownload
        && uploadRate == m_uploadRate && downloadRate == m_downloadRate
        && totalUpload == m_totalUpload && totalDownload == m_totalDownload
        && samples == m_trafficSamples) {
        return;
    }
    m_sessionUpload = sessionUpload;
    m_sessionDownload = sessionDownload;
    m_uploadRate = uploadRate;
    m_downloadRate = downloadRate;
    m_totalUpload = totalUpload;
    m_totalDownload = totalDownload;
    m_trafficSamples = samples;
    queueStateChanged();
}

void Controller::applyStatus(const QByteArray &body)
{
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    const bool helper = !object.isEmpty();
    const bool tun = object.value(QStringLiteral("tun")).toString() == QLatin1String("present");
    const bool core = object.value(QStringLiteral("mihomo")).toString() == QLatin1String("running");
    QString config = object.value(QStringLiteral("config")).toString();
    if (config.isEmpty()) {
        config = QStringLiteral("ok");
    }
    const QString helperVersion = object.value(QStringLiteral("helperVersion")).toString();
    const QString apiVersion = object.value(QStringLiteral("apiVersion")).toString();
    QString coreVersion = object.value(QStringLiteral("coreVersion")).toString();
    if (object.value(QStringLiteral("corePlace")).toString() == QLatin1String("data") && !coreVersion.isEmpty()) {
        coreVersion += QStringLiteral(" · 数据目录");
    }
    const bool mismatch = !helperVersion.isEmpty() && (helperVersion != QLatin1String(kAppVersion) || apiVersion != QLatin1String(kApiVersion));
    const bool missingVersion = helper && helperVersion.isEmpty();
    const QString versionText = QStringLiteral("助手 %1 · API %2 · 核心 %3").arg(
        helperVersion.isEmpty() ? QStringLiteral("—") : helperVersion,
        apiVersion.isEmpty() ? QStringLiteral("—") : apiVersion,
        coreVersion.isEmpty() ? QStringLiteral("—") : coreVersion);
    if (tun) {
        m_configBroken = false;
    }
    const bool blocked = object.value(QStringLiteral("network")).toString() == QLatin1String("blocked");
    const bool killOn = object.value(QStringLiteral("killSwitch")).toString() == QLatin1String("on");
    const bool allowOn = object.value(QStringLiteral("allowLan")).toString() == QLatin1String("on");
    const int lanPort = object.value(QStringLiteral("lanPort")).toString().toInt();
    if (helper == m_helperRunning && tun == m_tunRunning && core == m_coreRunning && config == m_configState && (mismatch || missingVersion) == m_versionMismatch && versionText == m_versionText && blocked == m_networkBlocked && killOn == m_killSwitch && allowOn == m_allowLan && lanPort == m_lanPort) {
        return;
    }
    m_helperRunning = helper;
    m_tunRunning = tun;
    m_coreRunning = core;
    m_configState = config;
    m_versionMismatch = mismatch || missingVersion;
    m_networkBlocked = blocked;
    m_killSwitch = killOn;
    m_allowLan = allowOn;
    m_lanPort = lanPort;
    m_versionText = versionText;
    queueStateChanged();
}

void Controller::applyProfile(const QByteArray &body)
{
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    const QJsonObject current = object.value(QStringLiteral("current")).toObject();
    const QJsonObject candidate = object.value(QStringLiteral("candidate")).toObject();
    QString text = QStringLiteral("当前配置：");
    text += current.value(QStringLiteral("name")).toString(QStringLiteral("无"));
    if (!current.value(QStringLiteral("host")).toString().isEmpty()) {
        text += QStringLiteral("（") + current.value(QStringLiteral("host")).toString() + QStringLiteral("）");
    }
    const QString candidateState = candidate.value(QStringLiteral("state")).toString();
    if (candidateState == QLatin1String("validated")) {
        text += QStringLiteral("\n待激活：") + candidate.value(QStringLiteral("name")).toString();
        if (!candidate.value(QStringLiteral("host")).toString().isEmpty()) {
            text += QStringLiteral("（") + candidate.value(QStringLiteral("host")).toString() + QStringLiteral("）");
        }
    } else if (candidateState == QLatin1String("failed")) {
        text += QStringLiteral("\n校验失败，当前配置未替换");
    }
    const QString ruleTemplate = object.value(QStringLiteral("ruleTemplate")).toString();
    text += QStringLiteral("\n规则：") + ruleTemplateLabel(ruleTemplate);
    if (m_profileText == text && m_ruleTemplate == ruleTemplate) {
        return;
    }
    m_profileText = text;
    m_ruleTemplate = ruleTemplate;
    queueStateChanged();
}
