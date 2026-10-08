#ifndef RAYUT_CONTROLLER_H
#define RAYUT_CONTROLLER_H

#include <QByteArray>
#include <QObject>
#include <QString>
#include <QUrl>
#include <QVariant>

class QNetworkAccessManager;

class Controller : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool helperRunning READ helperRunning NOTIFY stateChanged)
    Q_PROPERTY(bool tunRunning READ tunRunning NOTIFY stateChanged)
    Q_PROPERTY(bool versionMismatch READ versionMismatch NOTIFY stateChanged)
    Q_PROPERTY(QString summary READ summary NOTIFY stateChanged)
    Q_PROPERTY(QString message READ message NOTIFY stateChanged)
    Q_PROPERTY(QString profileText READ profileText NOTIFY stateChanged)
    Q_PROPERTY(QString versionText READ versionText NOTIFY stateChanged)
    Q_PROPERTY(QVariantList proxyGroups READ proxyGroups NOTIFY stateChanged)
    Q_PROPERTY(QVariantList sessionLogs READ sessionLogs NOTIFY stateChanged)
    Q_PROPERTY(QVariantList sessionConnections READ sessionConnections NOTIFY stateChanged)

public:
    explicit Controller(QObject *parent = nullptr);

    bool helperRunning() const;
    bool tunRunning() const;
    bool versionMismatch() const;
    QString summary() const;
    QString message() const;
    QString profileText() const;
    QString versionText() const;
    QVariantList proxyGroups() const;
    QVariantList sessionLogs() const;
    QVariantList sessionConnections() const;

    Q_INVOKABLE void refresh();
    Q_INVOKABLE void startHelper(QString password);
    Q_INVOKABLE void enableTun();
    Q_INVOKABLE void disableTun();
    Q_INVOKABLE void toggleProxy();
    Q_INVOKABLE void importContent(const QString &content);
    Q_INVOKABLE void importURL(const QString &url);
    Q_INVOKABLE void activateProfile();
    Q_INVOKABLE void refreshProfile();
    Q_INVOKABLE void refreshGroups();
    Q_INVOKABLE void selectProxy(const QString &group, const QString &name);
    Q_INVOKABLE void testDelay(const QString &name);
    Q_INVOKABLE void refreshSession();
    Q_INVOKABLE void importFromImage(const QUrl &url);

signals:
    void stateChanged();

private:
    bool readToken();
    bool request(const QString &method, const QString &path, const QByteArray &payload, QByteArray *response, int timeoutMs);
    void setMessage(const QString &message);
    void queueStateChanged();
    void applyStatus(const QByteArray &body);
    void applyProfile(const QByteArray &body);
    void applyGroups(const QByteArray &body);
    void applyLogs(const QByteArray &body);
    void applyConnections(const QByteArray &body);
    void postProfile(const QString &path, const QByteArray &payload, const QString &success);
    QString messageFor(const QString &code) const;

    QNetworkAccessManager *m_network;
    QString m_token;
    QString m_message;
    QString m_profileText;
    QString m_versionText;
    QString m_configState;
    bool m_helperRunning;
    bool m_tunRunning;
    bool m_coreRunning;
    bool m_configBroken;
    bool m_versionMismatch;
    bool m_stateQueued;
    QVariantList m_proxyGroups;
    QVariantList m_sessionLogs;
    QVariantList m_sessionConnections;
};

#endif
