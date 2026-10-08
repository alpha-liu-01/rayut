#ifndef RAYUT_CONTROLLER_H
#define RAYUT_CONTROLLER_H

#include <QAbstractListModel>
#include <QHash>
#include <QByteArray>
#include <QObject>
#include <QString>
#include <QStringList>
#include <QUrl>
#include <QVariant>

class QNetworkAccessManager;

// One row per line so the editor only builds the lines on screen.
// A single text control was laying out all 14k lines and the page stayed at a few frames per second.
class EditLineModel : public QAbstractListModel {
    Q_OBJECT

public:
    enum Roles { LineRole = Qt::UserRole + 1 };

    explicit EditLineModel(QObject *parent = nullptr);

    int rowCount(const QModelIndex &parent = QModelIndex()) const override;
    QVariant data(const QModelIndex &index, int role = Qt::DisplayRole) const override;
    QHash<int, QByteArray> roleNames() const override;

    void setDocument(const QString &text);
    QString document() const;
    void setLine(int row, const QString &text);
    void splitLine(int row, int cursor, int *focusRow, int *focusColumn);
    void joinLine(int row, int *focusRow, int *focusColumn);

private:
    QStringList m_lines;
};

class Controller : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool helperRunning READ helperRunning NOTIFY stateChanged)
    Q_PROPERTY(bool tunRunning READ tunRunning NOTIFY stateChanged)
    Q_PROPERTY(bool versionMismatch READ versionMismatch NOTIFY stateChanged)
    Q_PROPERTY(QString summary READ summary NOTIFY stateChanged)
    Q_PROPERTY(QString message READ message NOTIFY stateChanged)
    Q_PROPERTY(QString profileText READ profileText NOTIFY stateChanged)
    Q_PROPERTY(QString ruleTemplate READ ruleTemplate NOTIFY stateChanged)
    Q_PROPERTY(QString versionText READ versionText NOTIFY stateChanged)
    Q_PROPERTY(QVariantList proxyGroups READ proxyGroups NOTIFY stateChanged)
    Q_PROPERTY(QVariantList sessionLogs READ sessionLogs NOTIFY stateChanged)
    Q_PROPERTY(QVariantList sessionConnections READ sessionConnections NOTIFY stateChanged)
    Q_PROPERTY(EditLineModel *editLines READ editLines CONSTANT)
    Q_PROPERTY(int editFocusRow READ editFocusRow NOTIFY editFocusChanged)
    Q_PROPERTY(int editFocusColumn READ editFocusColumn NOTIFY editFocusChanged)
    Q_PROPERTY(QVariantList editProxies READ editProxies NOTIFY stateChanged)

public:
    explicit Controller(QObject *parent = nullptr);

    bool helperRunning() const;
    bool tunRunning() const;
    bool versionMismatch() const;
    QString summary() const;
    QString message() const;
    QString profileText() const;
    QString ruleTemplate() const;
    QString versionText() const;
    QVariantList proxyGroups() const;
    QVariantList sessionLogs() const;
    QVariantList sessionConnections() const;
    EditLineModel *editLines() const;
    int editFocusRow() const;
    int editFocusColumn() const;
    QVariantList editProxies() const;

    Q_INVOKABLE void refresh();
    Q_INVOKABLE void startHelper(QString password);
    Q_INVOKABLE void enableTun();
    Q_INVOKABLE void disableTun();
    Q_INVOKABLE void toggleProxy();
    Q_INVOKABLE void importContent(const QString &content);
    Q_INVOKABLE void importURL(const QString &url);
    Q_INVOKABLE void activateProfile();
    Q_INVOKABLE void refreshProfile();
    Q_INVOKABLE void applyRuleTemplate(const QString &id);
    Q_INVOKABLE void refreshGroups();
    Q_INVOKABLE void selectProxy(const QString &group, const QString &name);
    Q_INVOKABLE void testDelay(const QString &name);
    Q_INVOKABLE void refreshSession();
    Q_INVOKABLE void importFromImage(const QUrl &url);
    Q_INVOKABLE void loadProfileDocument();
    Q_INVOKABLE bool previewProfile();
    Q_INVOKABLE bool applyProxyEdit(int index, const QString &name, const QString &type, const QString &server, int port, const QString &network, bool tls, bool udp, const QString &secret);
    Q_INVOKABLE void saveProfileText();
    Q_INVOKABLE void setEditLine(int row, const QString &text);
    Q_INVOKABLE void splitEditLine(int row, int cursor);
    Q_INVOKABLE void joinEditLine(int row);

signals:
    void stateChanged();
    void editFocusChanged();

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
    void applyEditDocument(const QByteArray &body);
    void postProfile(const QString &path, const QByteArray &payload, const QString &success);
    QString messageFor(const QString &code) const;

    QNetworkAccessManager *m_network;
    QString m_token;
    QString m_message;
    QString m_profileText;
    QString m_ruleTemplate;
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
    EditLineModel *m_editLines;
    int m_editFocusRow;
    int m_editFocusColumn;
    QVariantList m_editProxies;
};

#endif
