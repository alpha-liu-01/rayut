set(PLUGIN "Rayut")
set(CMAKE_AUTOMOC ON)

find_package(Qt5Gui REQUIRED)
find_package(Qt5Qml REQUIRED)
find_package(Qt5Quick REQUIRED)
find_package(Qt5Network REQUIRED)

qt5_add_resources(NOTICE_RESOURCES notices.qrc)

add_library(${PLUGIN} MODULE
    controller.cpp
    plugin.cpp
    qrscan.c
    qrdraw.c
    ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/qrcodegen/qrcodegen.c
    ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/quirc/quirc.c
    ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/quirc/decode.c
    ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/quirc/identify.c
    ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/quirc/version_db.c
    ${NOTICE_RESOURCES}
)
target_include_directories(${PLUGIN} PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/quirc
    ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/qrcodegen
)
set_target_properties(${PLUGIN} PROPERTIES LIBRARY_OUTPUT_DIRECTORY ${PLUGIN})
target_link_libraries(${PLUGIN} Qt5::Gui Qt5::Qml Qt5::Quick Qt5::Network)

execute_process(
    COMMAND dpkg-architecture -qDEB_HOST_MULTIARCH
    OUTPUT_VARIABLE ARCH_TRIPLET
    OUTPUT_STRIP_TRAILING_WHITESPACE
)

set(QT_IMPORTS_DIR "/lib/${ARCH_TRIPLET}")
install(TARGETS ${PLUGIN} DESTINATION ${QT_IMPORTS_DIR}/${PLUGIN}/)
install(FILES qmldir DESTINATION ${QT_IMPORTS_DIR}/${PLUGIN}/)
