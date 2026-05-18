# Hermetic ARM64 Linux toolchain backed by zig's bundled clang.
#
# `zig cc` / `zig c++` are multi-call entry points: `zig` is the program,
# the next argv is the sub-command. CMake doesn't model that natively, so
# we point CMAKE_<LANG>_COMPILER at zig itself and pre-bake `cc` / `c++`
# as the first compiler arg via _COMPILER_ARG1. The target triple
# (`aarch64-linux-gnu.2.28`) is pinned to the same glibc baseline the
# hermetic_cc_toolchain registers for the CLI side.
#
# ABI note: zig's libc++ is statically linked into the produced .so/.exe
# by default, sidestepping the gcc-13 / Qualcomm Linux CXXABI_1.3.15 issue
# that motivated the gcc-only toolchain (#458).

set(CMAKE_SYSTEM_NAME Linux)
set(CMAKE_SYSTEM_PROCESSOR aarch64)

# ZIG_BIN is injected by Bazel's cmake_preset_build rule; it must survive
# CMake's `try_compile` recursion, which re-includes this toolchain file
# in a sub-build *without* the original -D arguments. Reading it from the
# environment (which CMake propagates) keeps the value available there.
if(NOT DEFINED ZIG_BIN)
    if(DEFINED ENV{ZIG_BIN})
        set(ZIG_BIN "$ENV{ZIG_BIN}")
    else()
        message(FATAL_ERROR
            "ZIG_BIN must be set via -DZIG_BIN=… or the ZIG_BIN env var. "
            "It is normally injected by Bazel's cmake_preset_build rule.")
    endif()
endif()
# Re-export so try_compile sub-invocations see it too.
set(ENV{ZIG_BIN} "${ZIG_BIN}")

set(_zig_target "aarch64-linux-gnu.2.28")

set(CMAKE_C_COMPILER "${ZIG_BIN}")
set(CMAKE_C_COMPILER_ARG1 "cc")
set(CMAKE_CXX_COMPILER "${ZIG_BIN}")
set(CMAKE_CXX_COMPILER_ARG1 "c++")
set(CMAKE_ASM_COMPILER "${ZIG_BIN}")
set(CMAKE_ASM_COMPILER_ARG1 "cc")

set(CMAKE_AR "${ZIG_BIN}" CACHE FILEPATH "")
# `zig ar` is a llvm-ar shim; CMake invokes AR directly so we wrap it via
# CMAKE_C_CREATE_STATIC_LIBRARY rather than passing the multi-call name.
# In practice CMAKE_AR alone is enough for ninja's archive recipe.

# Pin the target on every compile/link invocation. Adding it via flags
# (rather than a separate triple-prefixed driver) keeps the toolchain
# portable across zig versions.
set(_zig_flags "-target ${_zig_target}")
set(CMAKE_C_FLAGS_INIT "${_zig_flags}")
set(CMAKE_CXX_FLAGS_INIT "${_zig_flags}")
set(CMAKE_ASM_FLAGS_INIT "${_zig_flags}")
set(CMAKE_EXE_LINKER_FLAGS_INIT "${_zig_flags}")
set(CMAKE_SHARED_LINKER_FLAGS_INIT "${_zig_flags}")
set(CMAKE_MODULE_LINKER_FLAGS_INIT "${_zig_flags}")

# Skip CMake's compiler probe: the probe links a tiny program, and zig's
# linker driver dislikes some of CMake's default test invocations. Set
# language IDs to silence the probe; `try_compile` still works because
# CMAKE_<LANG>_COMPILER_WORKS gates that.
set(CMAKE_C_COMPILER_WORKS TRUE)
set(CMAKE_CXX_COMPILER_WORKS TRUE)

message(STATUS "Using hermetic zig toolchain: ${ZIG_BIN} (target ${_zig_target})")
