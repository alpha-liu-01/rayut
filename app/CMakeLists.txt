cmake_minimum_required(VERSION 3.0.0)
project(rayut C CXX)

set(CMAKE_AUTOMOC ON)
find_package(Qt5Core REQUIRED)
find_package(Qt5Gui REQUIRED)
find_package(Qt5Qml REQUIRED)
find_package(Qt5Quick REQUIRED)
find_package(Qt5QuickControls2 REQUIRED)

set(PROJECT_NAME "rayut")
configure_file(manifest.json.in ${CMAKE_CURRENT_BINARY_DIR}/manifest.json)
install(FILES ${CMAKE_CURRENT_BINARY_DIR}/manifest.json DESTINATION ${CMAKE_INSTALL_PREFIX})
install(FILES rayut.apparmor DESTINATION ${CMAKE_INSTALL_PREFIX})
install(FILES rayut.desktop DESTINATION ${CMAKE_INSTALL_PREFIX})
install(FILES assets/logo.svg DESTINATION assets)

qt5_add_resources(QT_RESOURCES qml/qml.qrc)
add_executable(${PROJECT_NAME} main.cpp ${QT_RESOURCES})
target_link_libraries(${PROJECT_NAME} Qt5::Gui Qt5::Qml Qt5::Quick Qt5::QuickControls2)
install(TARGETS ${PROJECT_NAME} RUNTIME DESTINATION ${CMAKE_INSTALL_PREFIX})

add_subdirectory(plugins/Rayut)
