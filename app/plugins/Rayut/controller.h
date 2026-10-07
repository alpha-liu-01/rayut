#ifndef RAYUT_CONTROLLER_H
#define RAYUT_CONTROLLER_H

#include <QObject>
#include <QString>

class QNetworkAccessManager;

class Controller : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool helperRunning READ helperRunning NOTIFY stateChanged)
    Q_PROPERTY(bool tunRunning READ tunRunning NOTIFY stateChanged)
    Q_PROPERTY(QString summary READ summary NOTIFY stateChanged)
    Q_PROPERTY(QString message READ message NOTIFY stateChanged)

public:
    explicit Controller(QObject *parent = nullptr);

    bool helperRunning() const;
    bool tunRunning() const;
    QString summary() const;
    QString message() const;

    Q_INVOKABLE void refresh();
    Q_INVOKABLE void startHelper(QString password);
    Q_INVOKABLE void enableTun();
    Q_INVOKABLE void disableTun();

signals:
    void stateChanged();

private:
    bool readToken();
    bool call(const QString &method, const QString &path);
    void setMessage(const QString &message);
    void applyStatus(const QByteArray &body);

    QNetworkAccessManager *m_network;
    QString m_token;
    QString m_message;
    bool m_helperRunning;
    bool m_tunRunning;
};

#endif
