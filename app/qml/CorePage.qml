import QtQuick 2.7
import Lomiri.Components 1.3
import QtQuick.Layouts 1.3
import Rayut 1.0

Page {
    id: coreRoot

    header: PageHeader {
        id: coreHeader
        title: qsTr("Core")
    }

    Component.onCompleted: Controller.refreshCores()

    Flickable {
        anchors {
            top: coreHeader.bottom
            left: parent.left
            right: parent.right
            bottom: parent.bottom
        }
        contentWidth: width
        contentHeight: column.height + units.gu(4)
        clip: true

        Column {
            id: column
            x: units.gu(2)
            y: units.gu(2)
            width: coreRoot.width - units.gu(4)
            spacing: units.gu(2)

            Label {
                width: parent.width
                wrapMode: Text.Wrap
                text: qsTr("The proxy core is the copy shipped in this app. Rayut does not download another one.")
            }

            Repeater {
                model: Controller.coreReleases

                delegate: Column {
                    width: column.width
                    spacing: units.gu(1)

                    Label {
                        width: parent.width
                        text: modelData.tag
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: modelData.license
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: modelData.source
                    }
                }
            }
        }
    }
}
