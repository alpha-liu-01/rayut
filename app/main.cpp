#include <QGuiApplication>
#include <QQmlEngine>
#include <QQuickView>
#include <QUrl>

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
    app.setApplicationName(QStringLiteral("rayut.rayut"));

    QQuickView view;
    view.setSource(QUrl(QStringLiteral("qrc:/Main.qml")));
    view.setResizeMode(QQuickView::SizeRootObjectToView);
    QObject::connect(view.engine(), &QQmlEngine::quit, &app, &QGuiApplication::quit);
    view.show();

    return app.exec();
}
