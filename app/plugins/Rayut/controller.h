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
    Q_PROPERTY(bool killSwitch READ killSwitch NOTIFY stateChanged)
    Q_PROPERTY(bool networkBlocked READ networkBlocked NOTIFY stateChanged)
    Q_PROPERTY(QString summary READ summary NOTIFY stateChanged)
    Q_PROPERTY(QString message READ message NOTIFY stateChanged)
    Q_PROPERTY(QString profileText READ profileText NOTIFY stateChanged)
    Q_PROPERTY(QString ruleTemplate READ ruleTemplate NOTIFY stateChanged)
    Q_PROPERTY(QString versionText READ versionText NOTIFY stateChanged)
    Q_PROPERTY(QVariantList proxyGroups READ proxyGroups NOTIFY stateChanged)
    Q_PROPERTY(QVariantList sessionLogs READ sessionLogs NOTIFY stateChanged)
    Q_PROPERTY(QVariantList sessionConnections READ sessionConnections NOTIFY stateChanged)
    Q_PROPERTY(qint64 sessionUpload READ sessionUpload NOTIFY stateChanged)
    Q_PROPERTY(qint64 sessionDownload READ sessionDownload NOTIFY stateChanged)
    Q_PROPERTY(qint64 uploadRate READ uploadRate NOTIFY stateChanged)
    Q_PROPERTY(qint64 downloadRate READ downloadRate NOTIFY stateChanged)
    Q_PROPERTY(qint64 totalUpload READ totalUpload NOTIFY stateChanged)
    Q_PROPERTY(qint64 totalDownload READ totalDownload NOTIFY stateChanged)
    Q_PROPERTY(QVariantList trafficSamples READ trafficSamples NOTIFY stateChanged)
    Q_PROPERTY(QVariantList profileGroups READ profileGroups NOTIFY stateChanged)
    Q_PROPERTY(QVariantList groupNodes READ groupNodes NOTIFY stateChanged)
    Q_PROPERTY(QVariantList proxySelectors READ proxySelectors NOTIFY stateChanged)
    Q_PROPERTY(QString selectorName READ selectorName NOTIFY stateChanged)
    Q_PROPERTY(QString viewedGroup READ viewedGroup NOTIFY stateChanged)
    Q_PROPERTY(QString activeGroup READ activeGroup NOTIFY stateChanged)
    Q_PROPERTY(QString groupKind READ groupKind NOTIFY stateChanged)
    Q_PROPERTY(QString groupURL READ groupURL NOTIFY stateChanged)
    Q_PROPERTY(bool delayRunning READ delayRunning NOTIFY stateChanged)
    Q_PROPERTY(int delayDone READ delayDone NOTIFY stateChanged)
    Q_PROPERTY(int delayTotal READ delayTotal NOTIFY stateChanged)
    Q_PROPERTY(EditLineModel *editLines READ editLines CONSTANT)
    Q_PROPERTY(int editFocusRow READ editFocusRow NOTIFY editFocusChanged)
    Q_PROPERTY(int editFocusColumn READ editFocusColumn NOTIFY editFocusChanged)
    Q_PROPERTY(QVariantList editProxies READ editProxies NOTIFY stateChanged)

public:
    explicit Controller(QObject *parent = nullptr);

    bool helperRunning() const;
    bool tunRunning() const;
    bool versionMismatch() const;
    bool killSwitch() const;
    bool networkBlocked() const;
    QString summary() const;
    QString message() const;
    QString profileText() const;
    QString ruleTemplate() const;
    QString versionText() const;
    QVariantList proxyGroups() const;
    QVariantList sessionLogs() const;
    QVariantList sessionConnections() const;
    qint64 sessionUpload() const;
    qint64 sessionDownload() const;
    qint64 uploadRate() const;
    qint64 downloadRate() const;
    qint64 totalUpload() const;
    qint64 totalDownload() const;
    QVariantList trafficSamples() const;
    QVariantList profileGroups() const;
    QVariantList groupNodes() const;
    QVariantList proxySelectors() const;
    QString selectorName() const;
    QString viewedGroup() const;
    QString activeGroup() const;
    QString groupKind() const;
    QString groupURL() const;
    bool delayRunning() const;
    int delayDone() const;
    int delayTotal() const;
    EditLineModel *editLines() const;
    int editFocusRow() const;
    int editFocusColumn() const;
    QVariantList editProxies() const;

    Q_INVOKABLE void refresh();
    Q_INVOKABLE void startHelper(QString password);
    Q_INVOKABLE void enableTun();
    Q_INVOKABLE void disableTun();
    Q_INVOKABLE void toggleProxy();
    Q_INVOKABLE void setKillSwitch(bool on);
    Q_INVOKABLE void refreshStatus();
    Q_INVOKABLE void importContent(const QString &content);
    Q_INVOKABLE void importURL(const QString &url);
    Q_INVOKABLE void importClipboard();
    Q_INVOKABLE void importClipboardScheme(const QString &scheme);
    Q_INVOKABLE void refreshProfileGroups();
    Q_INVOKABLE void showGroup(const QString &id);
    Q_INVOKABLE void useGroup(const QString &id);
    Q_INVOKABLE void refreshGroup(const QString &id);
    Q_INVOKABLE void deleteGroup(const QString &id);
    Q_INVOKABLE void clearGroup(const QString &id);
    Q_INVOKABLE void deleteNode(const QString &id, int index);
    Q_INVOKABLE void selectNode(const QString &id, const QString &group, const QString &name);
    Q_INVOKABLE void showSelector(const QString &name);
    Q_INVOKABLE void testNode(const QString &id, const QString &name);
    Q_INVOKABLE void testGroup(const QString &id);
    Q_INVOKABLE void pollGroupDelay(const QString &id);
    Q_INVOKABLE void exportGroup(const QString &id);
    Q_INVOKABLE void exportNode(const QString &id, const QString &name);
    Q_INVOKABLE void saveGroup(const QString &id, const QString &name, const QString &url);
    Q_INVOKABLE void loadGroupDetail(const QString &id);
    Q_INVOKABLE void restartProxy();
    Q_INVOKABLE void activateProfile();
    Q_INVOKABLE void refreshProfile();
    Q_INVOKABLE void applyRuleTemplate(const QString &id);
    Q_INVOKABLE void refreshGroups();
    Q_INVOKABLE void selectProxy(const QString &group, const QString &name);
    Q_INVOKABLE void testDelay(const QString &name);
    Q_INVOKABLE void refreshSession();
    Q_INVOKABLE void refreshTraffic();
    Q_INVOKABLE void importFromImage(const QUrl &url);
    Q_INVOKABLE void loadProfileDocument();
    Q_INVOKABLE void loadGroupDocument(const QString &id);
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
    bool request(const QString &method, const QString &path, const QByteArray &payload, QByteArray *response, int timeoutMs, bool allowTokenRefresh = true);
    void setMessage(const QString &message);
    void queueStateChanged();
    void applyStatus(const QByteArray &body);
    void applyProfile(const QByteArray &body);
    void applyGroups(const QByteArray &body);
    void applyLogs(const QByteArray &body);
    void applyConnections(const QByteArray &body);
    void applyTraffic(const QByteArray &body);
    void applyEditDocument(const QByteArray &body);
    void applyCatalog(const QByteArray &body, bool reloadNodes);
    void applyNodes(const QByteArray &body);
    void applySelectors(const QByteArray &body);
    void loadSelectors(const QString &id);
    void loadNodes(const QString &id);
    bool postCatalog(const QString &path, const QByteArray &payload, const QString &success, bool reloadNodes);
    bool postNodes(const QString &path, const QByteArray &payload, const QString &success);
    void copyExport(const QByteArray &body);
    QString groupPath(const QString &id, const QString &action) const;
    QString clipboardText() const;
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
    bool m_killSwitch;
    bool m_networkBlocked;
    bool m_stateQueued;
    QVariantList m_proxyGroups;
    QVariantList m_sessionLogs;
    QVariantList m_sessionConnections;
    qint64 m_sessionUpload;
    qint64 m_sessionDownload;
    qint64 m_uploadRate;
    qint64 m_downloadRate;
    qint64 m_totalUpload;
    qint64 m_totalDownload;
    QVariantList m_trafficSamples;
    EditLineModel *m_editLines;
    int m_editFocusRow;
    int m_editFocusColumn;
    QVariantList m_editProxies;
    QVariantList m_profileGroups;
    QVariantList m_groupNodes;
    QVariantList m_proxySelectors;
    QString m_selectorName;
    QString m_viewedGroup;
    QString m_activeGroup;
    QString m_groupKind;
    QString m_groupURL;
    QString m_editGroup;
    bool m_delayRunning;
    int m_delayDone;
    int m_delayTotal;
};

#endif
