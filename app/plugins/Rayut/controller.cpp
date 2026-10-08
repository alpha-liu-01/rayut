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

bool sameNodeShape(const QVariantMap &left, const QVariantMap &right)
{
    const QStringList keys = {
        QStringLiteral("index"),
        QStringLiteral("name"),
        QStringLiteral("type"),
        QStringLiteral("network"),
        QStringLiteral("selected"),
        QStringLiteral("shareable")
    };
    for (const QString &key : keys) {
        if (left.value(key) != right.value(key)) {
            return false;
        }
    }
    return true;
}

bool sameNodeList(const QVariantList &left, const QVariantList &right)
{
    if (left.size() != right.size()) {
        return false;
    }
    for (int i = 0; i < left.size(); ++i) {
        if (!sameNodeShape(left.at(i).toMap(), right.at(i).toMap())) {
            return false;
        }
    }
    return true;
}

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
        return QCoreApplication::translate("rayut", "Global proxy");
    }
    if (id == QLatin1String("lan")) {
        return QCoreApplication::translate("rayut", "Bypass LAN");
    }
    if (id == QLatin1String("lan-china")) {
        return QCoreApplication::translate("rayut", "Bypass LAN and mainland China");
    }
    return QCoreApplication::translate("rayut", "Built-in rules");
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
        return QCoreApplication::translate("rayut", "Helper is not running");
    }
    if (m_networkBlocked) {
        return QCoreApplication::translate("rayut", "Network is blocked, waiting to turn off or reconnect");
    }
    if (m_coreRunning && m_tunRunning) {
        return QCoreApplication::translate("rayut", "Proxy is on");
    }
    if (!m_coreRunning && !m_tunRunning) {
        if (m_configState == QLatin1String("missing") || m_configState == QLatin1String("error") || m_configBroken) {
            return QCoreApplication::translate("rayut", "Configuration error");
        }
        return QCoreApplication::translate("rayut", "Proxy is off");
    }
    return QCoreApplication::translate("rayut", "Core is not running");
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

QVariantMap Controller::nodeDelays() const
{
    return m_nodeDelays;
}

void Controller::rememberDelays(const QVariantList &nodes)
{
    for (const QVariant &value : nodes) {
        const QVariantMap node = value.toMap();
        m_nodeDelays.insert(node.value(QStringLiteral("name")).toString(), node.value(QStringLiteral("delay")).toInt());
    }
}

void Controller::noteDelayProgress()
{
    emit delayChanged();
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
        return QCoreApplication::translate("rayut", "Helper — · API — · Core —");
    }
    if (!m_versionMismatch) {
        return m_versionText;
    }
    return m_versionText + QLatin1Char('\n') + QCoreApplication::translate("rayut", "Version mismatch. Connect again.");
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
        setMessage(QCoreApplication::translate("rayut", "Helper is already running"));
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
        setMessage(QCoreApplication::translate("rayut", "Could not start sudo"));
        return;
    }
    process.write(secret);
    process.write("\n");
    secret.fill('\0');
    process.closeWriteChannel();
    if (!process.waitForFinished(30000)) {
        process.kill();
        setMessage(QCoreApplication::translate("rayut", "Startup timed out"));
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
    setMessage(QCoreApplication::translate("rayut", "Connection failed"));
}

void Controller::enableTun()
{
    if (m_versionMismatch) {
        setMessage(QCoreApplication::translate("rayut", "Version mismatch. Connect again."));
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
        setMessage(QCoreApplication::translate("rayut", "Version mismatch. Connect again."));
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
        setMessage(QCoreApplication::translate("rayut", "Version mismatch. Connect again."));
        return;
    }
    if (disconnectFirst && (m_tunRunning || m_coreRunning)) {
        setMessage(QCoreApplication::translate("rayut", "Disconnecting"));
        QByteArray body;
        if (!request(QStringLiteral("POST"), QStringLiteral("/v1/tun/disable"), QByteArray(), &body, 30000)) {
            return;
        }
        applyStatus(body);
    }
    if (m_tunRunning || m_coreRunning) {
        setMessage(QCoreApplication::translate("rayut", "Turn the proxy off first"));
        return;
    }
    m_coreBusy = true;
    setMessage(QCoreApplication::translate("rayut", "Checking and installing"));
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
        setMessage(QCoreApplication::translate("rayut", "Installed"));
    });
}

void Controller::setAllowLan(bool on)
{
    setMessage(QCoreApplication::translate("rayut", "Applying"));
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
            setMessage(QCoreApplication::translate("rayut", "Version mismatch. Connect again."));
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
    // Stored group name. Leave it untranslated so the same group is one row in every language.
    object.insert(QStringLiteral("name"), QStringLiteral("本地"));
    object.insert(QStringLiteral("content"), content);
    postCatalog(QStringLiteral("/v1/groups/import-content"), QJsonDocument(object).toJson(QJsonDocument::Compact), QCoreApplication::translate("rayut", "Imported"), true);
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
        setMessage(QCoreApplication::translate("rayut", "Could not read that picture"));
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
        setMessage(QCoreApplication::translate("rayut", "Could not read that picture"));
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
    postCatalog(QStringLiteral("/v1/groups/import-url"), QJsonDocument(object).toJson(QJsonDocument::Compact), QCoreApplication::translate("rayut", "Imported"), true);
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
        setMessage(QCoreApplication::translate("rayut", "The clipboard is empty"));
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
        setMessage(QCoreApplication::translate("rayut", "Copy a link of that type first"));
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
        setMessage(QCoreApplication::translate("rayut", "Turn the proxy off first"));
        return;
    }
    postCatalog(groupPath(id, QStringLiteral("use")), QByteArrayLiteral("{}"), QCoreApplication::translate("rayut", "Switched"), true);
}

void Controller::refreshGroup(const QString &id)
{
    postCatalog(groupPath(id, QStringLiteral("refresh")), QByteArrayLiteral("{}"), QCoreApplication::translate("rayut", "Refreshed"), true);
}

void Controller::deleteGroup(const QString &id)
{
    postCatalog(groupPath(id, QStringLiteral("delete")), QByteArrayLiteral("{}"), QCoreApplication::translate("rayut", "Deleted"), true);
}

void Controller::clearGroup(const QString &id)
{
    postNodes(groupPath(id, QStringLiteral("clear")), QByteArrayLiteral("{}"), QCoreApplication::translate("rayut", "Nodes deleted"));
}

void Controller::deleteNode(const QString &id, int index)
{
    QJsonObject object;
    object.insert(QStringLiteral("index"), index);
    postNodes(groupPath(id, QStringLiteral("node-delete")), QJsonDocument(object).toJson(QJsonDocument::Compact), QCoreApplication::translate("rayut", "Deleted"));
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
    setMessage(QCoreApplication::translate("rayut", "Selected"));
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
    noteDelayProgress();
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
    noteDelayProgress();
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
        return;
    }
    if (changed) {
        noteDelayProgress();
    }
}

void Controller::copyExport(const QByteArray &body)
{
    const QString text = QJsonDocument::fromJson(body).object().value(QStringLiteral("text")).toString();
    if (text.trimmed().isEmpty()) {
        setMessage(QCoreApplication::translate("rayut", "No share link to export"));
        return;
    }
    QClipboard *clipboard = QGuiApplication::clipboard();
    if (!clipboard) {
        setMessage(QCoreApplication::translate("rayut", "Request failed"));
        return;
    }
    clipboard->setText(text);
    setMessage(QCoreApplication::translate("rayut", "Copied"));
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
            showShareNote(QCoreApplication::translate("rayut", "No saved share link"));
            return true;
        }
        clearShare();
        return false;
    }
    const QString text = QJsonDocument::fromJson(body).object().value(QStringLiteral("text")).toString().trimmed();
    body.fill('\0');
    if (text.isEmpty()) {
        showShareNote(QCoreApplication::translate("rayut", "No saved share link"));
        return true;
    }
    unsigned char *pixels = nullptr;
    const int side = rayut_encode_qr(text.toUtf8().constData(), &pixels, 8);
    if (side < 1 || !pixels) {
        free(pixels);
        showShareNote(QCoreApplication::translate("rayut", "This link cannot be drawn as a code"));
        return true;
    }
    const QImage view(pixels, side, side, side, QImage::Format_Grayscale8);
    const QImage owned = view.copy();
    free(pixels);
    QByteArray png;
    QBuffer buffer(&png);
    if (!buffer.open(QIODevice::WriteOnly) || !owned.save(&buffer, "PNG")) {
        showShareNote(QCoreApplication::translate("rayut", "This link cannot be drawn as a code"));
        return true;
    }
    QClipboard *clipboard = QGuiApplication::clipboard();
    if (clipboard) {
        clipboard->setText(text);
        setMessage(QCoreApplication::translate("rayut", "Copied"));
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
    postCatalog(groupPath(id, QString()), QJsonDocument(object).toJson(QJsonDocument::Compact), QCoreApplication::translate("rayut", "Saved"), false);
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
        setMessage(QCoreApplication::translate("rayut", "Version mismatch. Connect again."));
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
        m_nodeDelays.clear();
        queueStateChanged();
        return;
    }
    QByteArray body;
    if (!request(QStringLiteral("GET"), groupPath(id, QStringLiteral("nodes")), QByteArray(), &body, 30000)) {
        m_groupNodes.clear();
        m_proxySelectors.clear();
        m_nodeDelays.clear();
        queueStateChanged();
        return;
    }
    const bool nodesChanged = applyNodes(body);
    const bool selectorsChanged = loadSelectors(id);
    if (!nodesChanged && !selectorsChanged) {
        noteDelayProgress();
    }
}

bool Controller::loadSelectors(const QString &id)
{
    if (id.isEmpty()) {
        m_proxySelectors.clear();
        m_selectorName.clear();
        m_nodeDelays.clear();
        queueStateChanged();
        return true;
    }
    QByteArray body;
    if (!request(QStringLiteral("GET"), groupPath(id, QStringLiteral("selectors")), QByteArray(), &body, 30000)) {
        m_proxySelectors.clear();
        m_nodeDelays.clear();
        queueStateChanged();
        return true;
    }
    return applySelectors(body);
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

bool Controller::applyNodes(const QByteArray &body)
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
    if (sameNodeList(m_groupNodes, list)) {
        rememberDelays(list);
        return false;
    }
    m_nodeDelays.clear();
    m_groupNodes = list;
    queueStateChanged();
    return true;
}

bool Controller::applySelectors(const QByteArray &body)
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
    QString nextSelector = m_selectorName;
    if (!keep) {
        if (savedHere) {
            nextSelector = saved;
        } else if (!trafficName.isEmpty()) {
            nextSelector = trafficName;
        } else {
            nextSelector = !firstManual.isEmpty() ? firstManual : firstName;
        }
    }
    bool same = nextSelector == m_selectorName && m_proxySelectors.size() == list.size();
    if (same) {
        for (int i = 0; i < list.size(); ++i) {
            const QVariantMap current = m_proxySelectors.at(i).toMap();
            const QVariantMap next = list.at(i).toMap();
            if (current.value(QStringLiteral("name")) != next.value(QStringLiteral("name"))
                || current.value(QStringLiteral("type")) != next.value(QStringLiteral("type"))
                || current.value(QStringLiteral("selectable")) != next.value(QStringLiteral("selectable"))
                || current.value(QStringLiteral("now")) != next.value(QStringLiteral("now"))
                || !sameNodeList(current.value(QStringLiteral("nodes")).toList(), next.value(QStringLiteral("nodes")).toList())) {
                same = false;
                break;
            }
        }
    }
    if (same) {
        for (const QVariant &value : list) {
            rememberDelays(value.toMap().value(QStringLiteral("nodes")).toList());
        }
        return false;
    }
    m_nodeDelays.clear();
    m_proxySelectors = list;
    m_selectorName = nextSelector;
    queueStateChanged();
    return true;
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
    postProfile(QStringLiteral("/v1/profiles/activate"), QByteArray(), QCoreApplication::translate("rayut", "Activated"));
}

void Controller::refreshProfile()
{
    postProfile(QStringLiteral("/v1/profiles/refresh"), QByteArray(), QCoreApplication::translate("rayut", "Checked. The current profile was not replaced."));
}

void Controller::applyRuleTemplate(const QString &id)
{
    if (m_viewedGroup.isEmpty()) {
        setMessage(QCoreApplication::translate("rayut", "No such group"));
        return;
    }
    if (m_viewedGroup == m_activeGroup && m_tunRunning) {
        setMessage(QCoreApplication::translate("rayut", "Turn the proxy off first"));
        return;
    }
    QJsonObject object;
    object.insert(QStringLiteral("id"), id);
    postCatalog(groupPath(m_viewedGroup, QStringLiteral("template")), QJsonDocument(object).toJson(QJsonDocument::Compact), QCoreApplication::translate("rayut", "Rule template applied"), false);
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
        setMessage(QCoreApplication::translate("rayut", "No such group"));
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
    setMessage(QCoreApplication::translate("rayut", "Loaded the current profile"));
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
    setMessage(QCoreApplication::translate("rayut", "Written back to the text. Not saved yet."));
    return true;
}

void Controller::saveProfileText()
{
    if (m_editGroup.isEmpty()) {
        setMessage(QCoreApplication::translate("rayut", "No such group"));
        return;
    }
    if (m_editGroup == m_activeGroup && m_tunRunning) {
        setMessage(QCoreApplication::translate("rayut", "Turn the proxy off first"));
        return;
    }
    setMessage(QCoreApplication::translate("rayut", "Checking…"));
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
    setMessage(QCoreApplication::translate("rayut", "Saved"));
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
    setMessage(QCoreApplication::translate("rayut", "Selected"));
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
    setMessage(QCoreApplication::translate("rayut", "Testing delay"));
    const QString path = QStringLiteral("/v1/proxies/")
        + QString::fromUtf8(QUrl::toPercentEncoding(name))
        + QStringLiteral("/delay");
    QByteArray body;
    if (!request(QStringLiteral("POST"), path, QByteArray(), &body, 20000)) {
        return;
    }
    const int delay = QJsonDocument::fromJson(body).object().value(QStringLiteral("delay")).toInt();
    setMessage(QCoreApplication::translate("rayut", "Delay %1 ms").arg(delay));
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
        return QCoreApplication::translate("rayut", "Profile is missing");
    }
    if (code == QLatin1String("core missing")) {
        return QCoreApplication::translate("rayut", "Core is missing");
    }
    if (code == QLatin1String("empty")) {
        return QCoreApplication::translate("rayut", "The content is empty");
    }
    if (code == QLatin1String("too large")) {
        return QCoreApplication::translate("rayut", "The content is too large");
    }
    if (code == QLatin1String("invalid yaml") || code == QLatin1String("invalid config")) {
        return QCoreApplication::translate("rayut", "Configuration check failed");
    }
    if (code == QLatin1String("invalid link")) {
        return QCoreApplication::translate("rayut", "The link is incomplete");
    }
    if (code == QLatin1String("unrecognized link")) {
        return QCoreApplication::translate("rayut", "Unrecognized link");
    }
    if (code == QLatin1String("file scheme")) {
        return QCoreApplication::translate("rayut", "file addresses are not allowed");
    }
    if (code == QLatin1String("hook")) {
        return QCoreApplication::translate("rayut", "External programs are not allowed");
    }
    if (code == QLatin1String("allow-lan")) {
        return QCoreApplication::translate("rayut", "Opening the LAN is not allowed");
    }
    if (code == QLatin1String("external-controller") || code == QLatin1String("bind-address") || code == QLatin1String("secret")) {
        return QCoreApplication::translate("rayut", "The controller must stay on this phone");
    }
    if (code == QLatin1String("fetch failed") || code == QLatin1String("redirect")) {
        return QCoreApplication::translate("rayut", "Subscription download failed");
    }
    if (code == QLatin1String("no subscription")) {
        return QCoreApplication::translate("rayut", "This is not a subscription");
    }
    if (code == QLatin1String("group missing")) {
        return QCoreApplication::translate("rayut", "No such group");
    }
    if (code == QLatin1String("tun running")) {
        return QCoreApplication::translate("rayut", "Turn the proxy off first");
    }
    if (code == QLatin1String("disconnect first")) {
        return QCoreApplication::translate("rayut", "Disconnect the proxy first");
    }
    if (code == QLatin1String("unknown release")) {
        return QCoreApplication::translate("rayut", "That version is not in the list");
    }
    if (code == QLatin1String("hash mismatch")) {
        return QCoreApplication::translate("rayut", "Check failed. The old core was not replaced.");
    }
    if (code == QLatin1String("license mismatch")) {
        return QCoreApplication::translate("rayut", "License check failed. The old core was not replaced.");
    }
    if (code == QLatin1String("download failed")) {
        return QCoreApplication::translate("rayut", "Core download failed");
    }
    if (code == QLatin1String("invalid file")) {
        return QCoreApplication::translate("rayut", "Invalid file. The old core was not replaced.");
    }
    if (code == QLatin1String("tun held") || code == QLatin1String("already-running")) {
        return QCoreApplication::translate("rayut", "The old core still holds the network. A second one was refused.");
    }
    if (code == QLatin1String("no candidate")) {
        return QCoreApplication::translate("rayut", "No profile to activate");
    }
    if (code == QLatin1String("unknown template")) {
        return QCoreApplication::translate("rayut", "No such rule template");
    }
    if (code == QLatin1String("bad field")) {
        return QCoreApplication::translate("rayut", "Invalid field");
    }
    if (code == QLatin1String("core not running")) {
        return QCoreApplication::translate("rayut", "Core is not running");
    }
    if (code == QLatin1String("controller unavailable")) {
        return QCoreApplication::translate("rayut", "Turn the proxy off, then on again");
    }
    if (code == QLatin1String("not found")) {
        return QCoreApplication::translate("rayut", "No such node");
    }
    if (code == QLatin1String("not selectable")) {
        return QCoreApplication::translate("rayut", "This group cannot be chosen by hand");
    }
    if (code == QLatin1String("invalid name")) {
        return QCoreApplication::translate("rayut", "Invalid name");
    }
    if (code == QLatin1String("selection rejected")) {
        return QCoreApplication::translate("rayut", "Selection failed");
    }
    if (code == QLatin1String("delay failed")) {
        return QCoreApplication::translate("rayut", "Delay test failed");
    }
    if (code == QLatin1String("timeout")) {
        return QCoreApplication::translate("rayut", "Delay test timed out");
    }
    return QCoreApplication::translate("rayut", "Request failed");
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
            row.insert(QStringLiteral("delayText"), delay > 0 ? QString::number(delay) + QStringLiteral(" ms") : QCoreApplication::translate("rayut", "Not tested"));
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
        coreVersion = QCoreApplication::translate("rayut", "%1 · data directory").arg(coreVersion);
    }
    const bool mismatch = !helperVersion.isEmpty() && (helperVersion != QLatin1String(kAppVersion) || apiVersion != QLatin1String(kApiVersion));
    const bool missingVersion = helper && helperVersion.isEmpty();
    const QString versionText = QCoreApplication::translate("rayut", "Helper %1 · API %2 · Core %3").arg(
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
    QString text = QCoreApplication::translate("rayut", "Current profile: %1").arg(
        current.value(QStringLiteral("name")).toString(QCoreApplication::translate("rayut", "none", "empty profile name")));
    if (!current.value(QStringLiteral("host")).toString().isEmpty()) {
        text += QCoreApplication::translate("rayut", " (%1)").arg(current.value(QStringLiteral("host")).toString());
    }
    const QString candidateState = candidate.value(QStringLiteral("state")).toString();
    if (candidateState == QLatin1String("validated")) {
        text += QLatin1Char('\n') + QCoreApplication::translate("rayut", "Waiting to activate: %1").arg(candidate.value(QStringLiteral("name")).toString());
        if (!candidate.value(QStringLiteral("host")).toString().isEmpty()) {
            text += QCoreApplication::translate("rayut", " (%1)").arg(candidate.value(QStringLiteral("host")).toString());
        }
    } else if (candidateState == QLatin1String("failed")) {
        text += QLatin1Char('\n') + QCoreApplication::translate("rayut", "Check failed. The current profile was not replaced.");
    }
    const QString ruleTemplate = object.value(QStringLiteral("ruleTemplate")).toString();
    text += QLatin1Char('\n') + QCoreApplication::translate("rayut", "Rules: %1").arg(ruleTemplateLabel(ruleTemplate));
    if (m_profileText == text && m_ruleTemplate == ruleTemplate) {
        return;
    }
    m_profileText = text;
    m_ruleTemplate = ruleTemplate;
    queueStateChanged();
}
