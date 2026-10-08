import QtQuick 2.7
import Lomiri.Components 1.3
import Lomiri.Components.Popups 1.3
import Lomiri.Components.Extras.PamAuthentication 0.1
import QtQuick.Layouts 1.3
import Rayut 1.0

MainView {
    id: root
    objectName: "mainView"
    applicationName: "rayut.rayut"
    anchorToKeyboard: true

    width: units.gu(45)
    height: units.gu(75)

    PamAuthentication {
        id: pam
        serviceName: "sudo"
    }

    Component {
        id: authDialog

        Dialog {
            id: dialog
            title: "Authentication required"
            text: "Enter passcode or passphrase:"

            function submit() {
                var entered = passwordField.text
                passwordField.text = ""
                if (!pam.validatePasswordToken(entered)) {
                    failure.text = "Authentication failed"
                    return
                }
                PopupUtils.close(dialog)
                Controller.startHelper(entered)
            }

            TextField {
                id: passwordField
                placeholderText: "passcode or passphrase"
                echoMode: TextInput.Password
                inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
                onAccepted: dialog.submit()
            }

            Label {
                id: failure
                visible: text !== ""
                wrapMode: Text.Wrap
                color: theme.palette.normal.negative
            }

            Button {
                text: "Authenticate"
                color: theme.palette.normal.positive
                onClicked: dialog.submit()
            }

            Button {
                text: "Cancel"
                onClicked: {
                    passwordField.text = ""
                    PopupUtils.close(dialog)
                }
            }

            Component.onCompleted: passwordField.forceActiveFocus()
        }
    }

    PageStack {
        id: stack
        anchors.fill: parent

        Component.onCompleted: stack.push(homePage)
    }

    Component {
        id: homePage

        Page {
            header: PageHeader {
                id: header
                title: "Rayut"

                trailingActionBar.actions: [
                    Action {
                        iconName: "note"
                        text: "订阅"
                        onTriggered: stack.push(profilePage)
                    }
                ]
            }

        ColumnLayout {
            spacing: units.gu(2)
            anchors {
                margins: units.gu(2)
                top: header.bottom
                left: parent.left
                right: parent.right
                bottom: parent.bottom
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: Controller.summary
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                visible: Controller.profileText !== ""
                text: Controller.profileText
            }

            Button {
                Layout.fillWidth: true
                visible: !Controller.helperRunning
                text: "连接"
                onClicked: PopupUtils.open(authDialog, root)
            }

            Button {
                Layout.fillWidth: true
                visible: Controller.helperRunning && !Controller.tunRunning
                text: "打开"
                onClicked: Controller.enableTun()
            }

            Button {
                Layout.fillWidth: true
                visible: Controller.tunRunning
                text: "关闭"
                onClicked: Controller.disableTun()
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
        }
    }

    Component {
        id: profilePage

        Page {
            header: PageHeader {
                title: "订阅"
            }

            Flickable {
                id: flick
                anchors.fill: parent
                contentHeight: profileColumn.implicitHeight + units.gu(4)
                clip: true

                ColumnLayout {
                    id: profileColumn
                    width: parent.width - units.gu(4)
                    x: units.gu(2)
                    y: units.gu(2)
                    spacing: units.gu(2)

                    Label {
                        Layout.fillWidth: true
                        wrapMode: Text.Wrap
                        text: Controller.helperRunning ? Controller.profileText : "请先在首页连接"
                    }

                    TextField {
                        id: urlField
                        Layout.fillWidth: true
                        placeholderText: "https 订阅链接"
                        enabled: Controller.helperRunning
                        inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "导入链接"
                        enabled: Controller.helperRunning
                        onClicked: Controller.importURL(urlField.text)
                    }

                    TextArea {
                        id: yamlField
                        Layout.fillWidth: true
                        Layout.preferredHeight: units.gu(18)
                        placeholderText: "本地 YAML"
                        enabled: Controller.helperRunning
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "导入 YAML"
                        enabled: Controller.helperRunning
                        onClicked: Controller.importContent(yamlField.text)
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "刷新"
                        enabled: Controller.helperRunning
                        onClicked: Controller.refreshProfile()
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "激活"
                        enabled: Controller.helperRunning && !Controller.tunRunning
                        onClicked: Controller.activateProfile()
                    }

                    Label {
                        Layout.fillWidth: true
                        wrapMode: Text.Wrap
                        color: theme.palette.normal.backgroundText
                        text: Controller.message
                    }
                }
            }
        }
    }

    Component.onCompleted: Controller.refresh()
}
