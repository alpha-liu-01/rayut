#ifndef RAYUT_PLUGIN_H
#define RAYUT_PLUGIN_H

#include <QQmlExtensionPlugin>

class RayutPlugin : public QQmlExtensionPlugin {
    Q_OBJECT
    Q_PLUGIN_METADATA(IID "org.qt-project.Qt.QQmlExtensionInterface")

public:
    void registerTypes(const char *uri) override;
};

#endif
