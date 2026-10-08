import QtQuick 2.7
import Lomiri.Components 1.3
import Lomiri.Components.Popups 1.3
import QtQuick.Layouts 1.3
import Rayut 1.0

Page {
    id: groupsRoot

    property string selected: ""

    function groupName(id) {
        var groups = Controller.profileGroups
        for (var i = 0; i < groups.length; i++) {
            if (groups[i].id === id)
                return groups[i].name
        }
        return ""
    }

    function fillFields() {
        nameField.text = groupsRoot.groupName(groupsRoot.selected)
        urlField.text = Controller.groupURL
    }

    header: PageHeader {
        id: groupsHeader
        title: qsTr("Subscription groups")
    }

    Timer {
        id: detailLoad
        interval: 1
        onTriggered: {
            if (groupsRoot.selected === "")
                return
            Controller.loadGroupDetail(groupsRoot.selected)
            groupsRoot.fillFields()
        }
    }

    Timer {
        id: work
        interval: 1
        property string kind: ""
        onTriggered: {
            if (groupsRoot.selected === "")
                return
            if (kind === "save")
                Controller.saveGroup(groupsRoot.selected, nameField.text, Controller.groupKind === "subscription" ? urlField.text : "")
            else if (kind === "refresh")
                Controller.refreshGroup(groupsRoot.selected)
            else if (kind === "delete")
                Controller.deleteGroup(groupsRoot.selected)
            else if (kind === "show")
                Controller.showGroup(groupsRoot.selected)
        }
    }

    Component.onCompleted: {
        selected = Controller.viewedGroup
        detailLoad.start()
    }

    Connections {
        target: Controller
        onStateChanged: {
            var groups = Controller.profileGroups
            for (var i = 0; i < groups.length; i++) {
                if (groups[i].id === groupsRoot.selected)
                    return
            }
            groupsRoot.selected = Controller.viewedGroup
            if (groupsRoot.selected !== "")
                detailLoad.start()
            else
                groupsRoot.fillFields()
        }
    }

    ColumnLayout {
        anchors {
            top: groupsHeader.bottom
            left: parent.left
            right: parent.right
            bottom: parent.bottom
            margins: units.gu(2)
        }
        spacing: units.gu(1)

        ListView {
            Layout.fillWidth: true
            Layout.preferredHeight: units.gu(24)
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            model: Controller.profileGroups

            delegate: AbstractButton {
                width: parent.width
                height: units.gu(5)
                onClicked: {
                    groupsRoot.selected = modelData.id
                    work.kind = "show"
                    work.start()
                    detailLoad.start()
                }

                Rectangle {
                    anchors.fill: parent
                    color: modelData.id === groupsRoot.selected ? theme.palette.normal.foreground : "transparent"
                }

                Label {
                    anchors {
                        left: parent.left
                        right: parent.right
                        verticalCenter: parent.verticalCenter
                        leftMargin: units.gu(1)
                    }
                    elide: Text.ElideRight
                    color: modelData.id === groupsRoot.selected ? theme.palette.normal.foregroundText : theme.palette.normal.backgroundText
                    text: modelData.active
                          ? qsTr("%1 (%2), in use").arg(modelData.name).arg(modelData.count)
                          : qsTr("%1 (%2)").arg(modelData.name).arg(modelData.count)
                }
            }
        }

        TextField {
            id: nameField
            Layout.fillWidth: true
            placeholderText: qsTr("Group name")
            enabled: groupsRoot.selected !== ""
            inputMethodHints: Qt.ImhNoPredictiveText
        }

        TextField {
            id: urlField
            Layout.fillWidth: true
            visible: Controller.groupKind === "subscription"
            placeholderText: qsTr("Change the subscription URL only here")
            enabled: groupsRoot.selected !== ""
            inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
        }

        Button {
            Layout.fillWidth: true
            text: qsTr("Save")
            enabled: groupsRoot.selected !== ""
            onClicked: {
                work.kind = "save"
                work.start()
            }
        }

        Button {
            Layout.fillWidth: true
            text: qsTr("Refresh")
            visible: Controller.groupKind === "subscription"
            enabled: groupsRoot.selected !== ""
            onClicked: {
                work.kind = "refresh"
                work.start()
            }
        }

        Button {
            Layout.fillWidth: true
            text: qsTr("Delete the whole group")
            enabled: groupsRoot.selected !== ""
            onClicked: PopupUtils.open(deleteDialog, groupsRoot)
        }

        Label {
            Layout.fillWidth: true
            wrapMode: Text.Wrap
            color: theme.palette.normal.backgroundText
            text: Controller.message
        }

        Item {
            Layout.fillHeight: true
        }
    }

    Component {
        id: deleteDialog

        Dialog {
            id: deleteBox
            title: qsTr("Delete this group?")
            text: qsTr("A group in use cannot be deleted while the proxy is on.")

            Button {
                text: qsTr("Delete")
                color: theme.palette.normal.negative
                onClicked: {
                    PopupUtils.close(deleteBox)
                    work.kind = "delete"
                    work.start()
                }
            }

            Button {
                text: qsTr("Cancel")
                onClicked: PopupUtils.close(deleteBox)
            }
        }
    }
}
