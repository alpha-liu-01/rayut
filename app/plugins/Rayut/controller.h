#ifndef RAYUT_CONTROLLER_H
#define RAYUT_CONTROLLER_H

#include <QByteArray>
#include <QObject>
#include <QString>

class QNetworkAccessManager;

class Controller : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool helperRunning READ helperRunning NOTIFY stateChanged)
    Q_PROPERTY(bool tunRunning READ tunRunning NOTIFY stateChanged)
    Q_PROPERTY(QString summary READ summary NOTIFY stateChanged)
    Q_PROPERTY(QString message READ message NOTIFY stateChanged)
    Q_PROPERTY(QString profileText READ profileText NOTIFY stateChanged)

public:
    explicit Controller(QObject *parent = nullptr);

    bool helperRunning() const;
    bool tunRunning() const;
    QString summary() const;
    QString message() const;
    QString profileText() const;

    Q_INVOKABLE void refresh();
    Q_INVOKABLE void startHelper(QString password);
    Q_INVOKABLE void enableTun();
    Q_INVOKABLE void disableTun();
    Q_INVOKABLE void importContent(const QString &content);
    Q_INVOKABLE void importURL(const QString &url);
    Q_INVOKABLE void activateProfile();
    Q_INVOKABLE void refreshProfile();

signals:
    void stateChanged();

private:
    bool readToken();
    bool request(const QString &method, const QString &path, const QByteArray &payload, QByteArray *response, int timeoutMs);
    void setMessage(const QString &message);
    void applyStatus(const QByteArray &body);
    void applyProfile(const QByteArray &body);
    void postProfile(const QString &path, const QByteArray &payload, const QString &success);
    QString messageFor(const QString &code) const;

    QNetworkAccessManager *m_network;
    QString m_token;
    QString m_message;
    QString m_profileText;
    bool m_helperRunning;
    bool m_tunRunning;
};

#endif
