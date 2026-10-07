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

    Page {
        anchors.fill: parent

        header: PageHeader {
            id: header
            title: "Rayut"
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

    Component.onCompleted: Controller.refresh()
}
