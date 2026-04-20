file(GLOB BRIDGE_SRC *.cpp)

add_library(geniex SHARED ${BRIDGE_SRC})

target_compile_definitions(geniex PRIVATE GENIEX_VERSION="${GENIEX_VERSION}")
target_compile_definitions(geniex PRIVATE ML_SHARED ML_BUILD)

target_include_directories(geniex PUBLIC ${COMMON_INCLUDE})

if(GENIEX_MODEL_MANAGER)
    add_dependencies(geniex model_manager_build)
    target_link_libraries(geniex PRIVATE
        "${RUST_TARGET_DIR}/libgeniex_model.a"
    )
    target_include_directories(geniex PUBLIC
        "${CMAKE_SOURCE_DIR}/model-manager/include"
    )
    install(FILES "${CMAKE_SOURCE_DIR}/model-manager/include/ml_model.h"
        DESTINATION include)

    if(GENIEX_TEST)
        add_executable(test_model_manager
            "${CMAKE_SOURCE_DIR}/model-manager/tests/test_model_manager.c"
        )
        target_include_directories(test_model_manager PRIVATE
            ${COMMON_INCLUDE}
            "${CMAKE_SOURCE_DIR}/model-manager/include"
        )
        target_link_libraries(test_model_manager PRIVATE geniex)
        add_test(NAME test_model_manager COMMAND test_model_manager)
    endif()
endif()

install(TARGETS geniex
    LIBRARY DESTINATION lib
    RUNTIME DESTINATION lib)

install(FILES ${CMAKE_SOURCE_DIR}/include/ml.h DESTINATION include)
