#include <QCoreApplication>
#include <QDir>
#include <QGuiApplication>
#include <QLocale>
#include <QQmlEngine>
#include <QQuickView>
#include <QTranslator>
#include <QUrl>

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
    app.setApplicationName(QStringLiteral("rayut.rayut"));

    // Source strings are English. A rayut_<locale>.qm in translations/ is loaded
    // for the system locale, so another language is a new file, not a code change.
    QTranslator translator;
    const QString translations = QDir(QCoreApplication::applicationDirPath()).filePath(QStringLiteral("translations"));
    const QLocale locale;
    bool translated = translator.load(locale, QStringLiteral("rayut"), QStringLiteral("_"), translations);
    if (!translated && locale.language() == QLocale::Chinese) {
        translated = translator.load(QStringLiteral("rayut_zh_CN"), translations);
    }
    if (translated) {
        app.installTranslator(&translator);
    }

    QQuickView view;
    view.setSource(QUrl(QStringLiteral("qrc:/Main.qml")));
    view.setResizeMode(QQuickView::SizeRootObjectToView);
    QObject::connect(view.engine(), &QQmlEngine::quit, &app, &QGuiApplication::quit);
    view.show();

    return app.exec();
}
