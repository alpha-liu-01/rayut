#include "controller.h"

#include <QEventLoop>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QProcess>
#include <QTimer>
#include <QUrl>

namespace {
const char tokenPath[] = "/home/phablet/rayut-day2/client-token";
const char helperPath[] = "/home/phablet/rayut-day2/rayutd";
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
    QFile file(QString::fromLatin1(tokenPath));
    if (!file.open(QIODevice::ReadOnly)) {
        m_token.clear();
        return false;
    }
    m_token = QString::fromUtf8(file.readAll()).trimmed();
    return !m_token.isEmpty();
}

void Controller::refresh()
{
    if (!call(QStringLiteral("GET"), QStringLiteral("/v1/status"))) {
        const bool changed = m_helperRunning || m_tunRunning;
        m_helperRunning = false;
        m_tunRunning = false;
        if (changed) {
            emit stateChanged();
        }
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
        QString::fromLatin1(helperPath),
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
    if (call(QStringLiteral("POST"), QStringLiteral("/v1/tun/enable"))) {
        setMessage(QString());
    }
}

void Controller::disableTun()
{
    if (call(QStringLiteral("POST"), QStringLiteral("/v1/tun/disable"))) {
        setMessage(QString());
    }
}

bool Controller::call(const QString &method, const QString &path)
{
    if (m_token.isEmpty() && !readToken()) {
        return false;
    }

    QNetworkRequest request(QUrl(QStringLiteral("http://127.0.0.1:18771") + path));
    request.setRawHeader("Authorization", QByteArray("Bearer ") + m_token.toUtf8());
    QNetworkReply *reply = nullptr;
    if (method == QLatin1String("GET")) {
        reply = m_network->get(request);
    } else {
        reply = m_network->sendCustomRequest(request, method.toUtf8(), QByteArray());
    }

    QEventLoop loop;
    QTimer timer;
    timer.setSingleShot(true);
    QObject::connect(&timer, &QTimer::timeout, &loop, &QEventLoop::quit);
    QObject::connect(reply, &QNetworkReply::finished, &loop, &QEventLoop::quit);
    timer.start(30000);
    loop.exec();
    timer.stop();

    const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
    if (!reply->isFinished() || reply->error() != QNetworkReply::NoError || status < 200 || status >= 300) {
        reply->deleteLater();
        if (status == 401) {
            m_token.clear();
        }
        if (method != QLatin1String("GET")) {
            setMessage(QStringLiteral("请求失败"));
        }
        return false;
    }
    const QByteArray body = reply->readAll();
    reply->deleteLater();
    applyStatus(body);
    return true;
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
