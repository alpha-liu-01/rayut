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

set(RAYUT_CORE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/packaging/core")
if(NOT EXISTS "${RAYUT_CORE_DIR}/rayutd" OR NOT EXISTS "${RAYUT_CORE_DIR}/mihomo")
    message(FATAL_ERROR "packaging/core/rayutd and packaging/core/mihomo must exist before the Click build")
endif()
install(PROGRAMS "${RAYUT_CORE_DIR}/rayutd" "${RAYUT_CORE_DIR}/mihomo" DESTINATION bin)

qt5_add_resources(QT_RESOURCES qml/qml.qrc)
add_executable(${PROJECT_NAME} main.cpp ${QT_RESOURCES})
target_link_libraries(${PROJECT_NAME} Qt5::Gui Qt5::Qml Qt5::Quick Qt5::QuickControls2)
install(TARGETS ${PROJECT_NAME} RUNTIME DESTINATION ${CMAKE_INSTALL_PREFIX})

# Any translations/rayut_<locale>.ts is a language. rayut.ts is the empty template and is not installed.
file(GLOB RAYUT_TRANSLATIONS "${CMAKE_CURRENT_SOURCE_DIR}/translations/rayut_*.ts")
find_program(LRELEASE_EXECUTABLE NAMES lrelease lrelease-qt5)
if(LRELEASE_EXECUTABLE AND RAYUT_TRANSLATIONS)
    set(RAYUT_QM "")
    foreach(ts ${RAYUT_TRANSLATIONS})
        get_filename_component(RAYUT_TS_NAME "${ts}" NAME_WE)
        set(qm "${CMAKE_CURRENT_BINARY_DIR}/${RAYUT_TS_NAME}.qm")
        add_custom_command(
            OUTPUT "${qm}"
            COMMAND "${LRELEASE_EXECUTABLE}" "${ts}" -qm "${qm}"
            DEPENDS "${ts}"
            VERBATIM)
        list(APPEND RAYUT_QM "${qm}")
    endforeach()
    add_custom_target(rayut_translations ALL DEPENDS ${RAYUT_QM})
    install(FILES ${RAYUT_QM} DESTINATION translations)
else()
    file(GLOB RAYUT_QM "${CMAKE_CURRENT_SOURCE_DIR}/translations/rayut_*.qm")
    if(NOT RAYUT_QM)
        message(FATAL_ERROR "No translations/rayut_*.qm and lrelease was not found")
    endif()
    install(FILES ${RAYUT_QM} DESTINATION translations)
endif()

add_subdirectory(plugins/Rayut)
