#include "controller.h"

#include <QCoreApplication>
#include <QDir>
#include <QEventLoop>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QProcess>
#include <QStandardPaths>
#include <QTimer>
#include <QUrl>

namespace {
QString helperPath()
{
    return QDir(QCoreApplication::applicationDirPath()).filePath(QStringLiteral("bin/rayutd"));
}

QString tokenPath()
{
    const QString dir = QStandardPaths::writableLocation(QStandardPaths::AppConfigLocation);
    return QDir(dir).filePath(QStringLiteral("client-token"));
}
}

Controller::Controller(QObject *parent)
    : QObject(parent)
    , m_network(new QNetworkAccessManager(this))
    , m_helperRunning(false)
    , m_tunRunning(false)
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

QString Controller::summary() const
{
    if (!m_helperRunning) {
        return QStringLiteral("助手未运行");
    }
    if (m_tunRunning) {
        return QStringLiteral("全局代理已打开");
    }
    return QStringLiteral("全局代理已关闭");
}

QString Controller::message() const
{
    return m_message;
}

QString Controller::profileText() const
{
    return m_profileText;
}

void Controller::setMessage(const QString &message)
{
    if (m_message == message) {
        return;
    }
    m_message = message;
    emit stateChanged();
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
        const bool changed = m_helperRunning || m_tunRunning || !m_profileText.isEmpty();
        m_helperRunning = false;
        m_tunRunning = false;
        m_profileText.clear();
        if (changed) {
            emit stateChanged();
        }
        return;
    }
    applyStatus(body);
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
    refresh();
    if (m_helperRunning) {
        setMessage(QString());
        return;
    }
    setMessage(QStringLiteral("连接失败"));
}

void Controller::enableTun()
{
    QByteArray body;
    if (request(QStringLiteral("POST"), QStringLiteral("/v1/tun/enable"), QByteArray(), &body, 30000)) {
        applyStatus(body);
        setMessage(QString());
    }
}

void Controller::disableTun()
{
    QByteArray body;
    if (request(QStringLiteral("POST"), QStringLiteral("/v1/tun/disable"), QByteArray(), &body, 30000)) {
        applyStatus(body);
        setMessage(QString());
    }
}

void Controller::importContent(const QString &content)
{
    QJsonObject object;
    object.insert(QStringLiteral("name"), QStringLiteral("本地"));
    object.insert(QStringLiteral("content"), content);
    postProfile(QStringLiteral("/v1/profiles/import-content"), QJsonDocument(object).toJson(QJsonDocument::Compact), QStringLiteral("已校验，当前配置未替换"));
}

void Controller::importURL(const QString &url)
{
    QJsonObject object;
    object.insert(QStringLiteral("url"), url);
    postProfile(QStringLiteral("/v1/profiles/import-url"), QJsonDocument(object).toJson(QJsonDocument::Compact), QStringLiteral("已校验，当前配置未替换"));
}

void Controller::activateProfile()
{
    postProfile(QStringLiteral("/v1/profiles/activate"), QByteArray(), QStringLiteral("已激活"));
}

void Controller::refreshProfile()
{
    postProfile(QStringLiteral("/v1/profiles/refresh"), QByteArray(), QStringLiteral("已校验，当前配置未替换"));
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

bool Controller::request(const QString &method, const QString &path, const QByteArray &payload, QByteArray *response, int timeoutMs)
{
    if (m_token.isEmpty() && !readToken()) {
        return false;
    }

    QNetworkRequest request(QUrl(QStringLiteral("http://127.0.0.1:18771") + path));
    request.setRawHeader("Authorization", QByteArray("Bearer ") + m_token.toUtf8());
    if (!payload.isEmpty()) {
        request.setHeader(QNetworkRequest::ContentTypeHeader, QStringLiteral("application/json"));
    }
    QNetworkReply *reply = nullptr;
    if (method == QLatin1String("GET")) {
        reply = m_network->get(request);
    } else {
        reply = m_network->sendCustomRequest(request, method.toUtf8(), payload);
    }

    QEventLoop loop;
    QTimer timer;
    timer.setSingleShot(true);
    QObject::connect(&timer, &QTimer::timeout, &loop, &QEventLoop::quit);
    QObject::connect(reply, &QNetworkReply::finished, &loop, &QEventLoop::quit);
    timer.start(timeoutMs);
    loop.exec();
    timer.stop();

    const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
    const QByteArray body = reply->readAll();
    if (!reply->isFinished() || reply->error() != QNetworkReply::NoError || status < 200 || status >= 300) {
        reply->deleteLater();
        if (status == 401) {
            m_token.clear();
        }
        if (method != QLatin1String("GET")) {
            setMessage(messageFor(QString::fromUtf8(body).trimmed()));
        }
        return false;
    }
    reply->deleteLater();
    if (response) {
        *response = body;
    }
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
        return QStringLiteral("没有订阅地址");
    }
    if (code == QLatin1String("tun running")) {
        return QStringLiteral("请先关闭代理");
    }
    if (code == QLatin1String("no candidate")) {
        return QStringLiteral("没有可激活的配置");
    }
    return QStringLiteral("请求失败");
}

void Controller::applyStatus(const QByteArray &body)
{
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    const bool helper = !object.isEmpty();
    const bool tun = object.value(QStringLiteral("tun")).toString() == QLatin1String("present");
    if (helper == m_helperRunning && tun == m_tunRunning) {
        return;
    }
    m_helperRunning = helper;
    m_tunRunning = tun;
    emit stateChanged();
}

void Controller::applyProfile(const QByteArray &body)
{
    const QJsonObject object = QJsonDocument::fromJson(body).object();
    const QJsonObject current = object.value(QStringLiteral("current")).toObject();
    const QJsonObject candidate = object.value(QStringLiteral("candidate")).toObject();
    QString text = QStringLiteral("当前：");
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
    if (m_profileText == text) {
        return;
    }
    m_profileText = text;
    emit stateChanged();
}
