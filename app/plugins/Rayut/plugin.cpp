#include <QtQml>

#include "controller.h"
#include "plugin.h"

void RayutPlugin::registerTypes(const char *uri)
{
    qmlRegisterSingletonType<Controller>(uri, 1, 0, "Controller", [](QQmlEngine *, QJSEngine *) -> QObject * {
        return new Controller;
    });
}
