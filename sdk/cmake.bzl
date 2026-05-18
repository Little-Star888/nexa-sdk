"""Bazel rule that drives the SDK's CMake build via `cmake --preset`.

The action is `local + no-sandbox + no-cache` so the per-preset CMake
build directory lives in the source tree (`sdk/build-<preset>/`) and
ninja's incremental state survives across Bazel invocations. Hermeticity
is traded for the same edit-rebuild loop the manual `cmake --build`
workflow has.

Outputs (under `bazel-bin/sdk/cmake_out/`):

    libgeniex.so, geniex.h, llama_cpp/, qairt/, bin/

The output path is preset-independent on purpose; the *build* directory
still varies by preset so cross-compile targets keep separate ninja
state.
"""

def _cmake_preset_build_impl(ctx):
    preset = ctx.attr.preset
    out_dir = "cmake_out"

    out_lib = ctx.actions.declare_file(out_dir + "/libgeniex.so")
    out_hdr = ctx.actions.declare_file(out_dir + "/geniex.h")
    out_llama = ctx.actions.declare_directory(out_dir + "/llama_cpp")
    out_qairt = ctx.actions.declare_directory(out_dir + "/qairt")
    out_bin = ctx.actions.declare_directory(out_dir + "/bin")
    outputs = [out_lib, out_hdr, out_llama, out_qairt, out_bin]

    cmake_bin = ctx.file.cmake
    ninja_bin = ctx.file.ninja
    zig_bin = ctx.file.zig
    toolchain_file = ctx.file.toolchain_file
    # cmake needs its full tree at runtime (Modules/, Templates/, …); the
    # `cmake_runtime` attr carries that as a filegroup. zig also needs its
    # standard library shipped via `zig_runtime`.
    tools = depset(
        direct = [cmake_bin, ninja_bin, zig_bin, toolchain_file],
        transitive = [
            ctx.attr.cmake_runtime.files,
            ctx.attr.zig_runtime.files,
        ],
    )

    ctx.actions.run_shell(
        outputs = outputs,
        inputs = ctx.files.srcs,
        tools = tools,
        command = r"""
set -euo pipefail
CMAKE_BIN="$1"
NINJA_BIN="$2"
ZIG_BIN="$3"
TOOLCHAIN_FILE="$4"
PRESET="$5"
OUT_LIB="$PWD/$6"
OUT_HDR="$PWD/$7"
OUT_LLAMA="$PWD/$8"
OUT_QAIRT="$PWD/$9"
OUT_BIN="$PWD/${10}"

# Bazel labels resolve to execroot-relative paths; absolutize before
# we cd into the SDK source tree.
CMAKE_BIN="$PWD/$CMAKE_BIN"
NINJA_BIN="$PWD/$NINJA_BIN"
ZIG_BIN="$PWD/$ZIG_BIN"
TOOLCHAIN_FILE="$PWD/$TOOLCHAIN_FILE"

# Put ninja on PATH (cmake's preset uses Ninja generator and shells out
# to whatever `ninja` it finds). ZIG_BIN goes into the environment so
# CMake's try_compile sub-invocations (which discard -D flags) still see
# it via the toolchain file. ZIG_*_CACHE_DIR pin zig's own caches inside
# the persistent build tree so it doesn't try to write to HOME (which is
# unset in the action's environment).
export PATH="$(dirname "$NINJA_BIN"):$PATH"
export ZIG_BIN

# Operate on the real source tree (followed through Bazel's execroot
# symlink) so build-<preset>/ persists between actions for ninja.
SDK_DIR="$(realpath sdk)"
cd "$SDK_DIR"

# Pin zig's internal caches inside the persistent build tree. Without
# this zig falls back to $XDG_CACHE_HOME/$HOME/.cache/zig, both of which
# may be unset in Bazel's action environment, producing
# `error: AppDataDirUnavailable`.
export ZIG_GLOBAL_CACHE_DIR="$SDK_DIR/build-$PRESET/.zig-cache"
export ZIG_LOCAL_CACHE_DIR="$ZIG_GLOBAL_CACHE_DIR"
mkdir -p "$ZIG_GLOBAL_CACHE_DIR"

# Override the preset's CMAKE_TOOLCHAIN_FILE with the hermetic zig
# toolchain, force the Ninja generator, and pin the make program to the
# ninja Bazel shipped (the preset doesn't do either, so default would be
# Unix Makefiles searching for system `make`).
"$CMAKE_BIN" --preset "$PRESET" \
    -G Ninja \
    -DCMAKE_MAKE_PROGRAM="$NINJA_BIN" \
    -DCMAKE_TOOLCHAIN_FILE="$TOOLCHAIN_FILE" \
    -DZIG_BIN="$ZIG_BIN" >&2
"$CMAKE_BIN" --build "build-$PRESET" -j >&2

STAGE="$SDK_DIR/build-$PRESET/.bazel-stage"
rm -rf "$STAGE"
"$CMAKE_BIN" --install "build-$PRESET" --prefix "$STAGE" >&2

cp "$STAGE/lib/libgeniex.so" "$OUT_LIB"
cp "$STAGE/include/geniex.h" "$OUT_HDR"
if [ -d "$STAGE/lib/llama_cpp" ]; then cp -r "$STAGE/lib/llama_cpp/." "$OUT_LLAMA/"; fi
if [ -d "$STAGE/lib/qairt" ]; then cp -r "$STAGE/lib/qairt/." "$OUT_QAIRT/"; fi
if [ -d "$STAGE/bin" ]; then cp -r "$STAGE/bin/." "$OUT_BIN/"; fi
""",
        arguments = [
            cmake_bin.path,
            ninja_bin.path,
            zig_bin.path,
            toolchain_file.path,
            preset,
            out_lib.path,
            out_hdr.path,
            out_llama.path,
            out_qairt.path,
            out_bin.path,
        ],
        execution_requirements = {
            "no-sandbox": "1",
            "local": "1",
            "no-cache": "1",
            # cargo (model-manager) fetches crates on first build.
            "requires-network": "1",
        },
        progress_message = "CMake preset build (%s)" % preset,
        mnemonic = "CMakePresetBuild",
        use_default_shell_env = True,
    )

    return [
        DefaultInfo(files = depset(outputs)),
        OutputGroupInfo(
            shared_library = depset([out_lib]),
            header = depset([out_hdr]),
            plugins = depset([out_llama, out_qairt]),
            bin = depset([out_bin]),
        ),
    ]

cmake_preset_build = rule(
    implementation = _cmake_preset_build_impl,
    attrs = {
        "preset": attr.string(mandatory = True),
        "srcs": attr.label_list(allow_files = True),
        "cmake": attr.label(
            mandatory = True,
            allow_single_file = True,
            cfg = "exec",
        ),
        "cmake_runtime": attr.label(
            mandatory = True,
            cfg = "exec",
        ),
        "ninja": attr.label(
            mandatory = True,
            allow_single_file = True,
            cfg = "exec",
        ),
        "zig": attr.label(
            mandatory = True,
            allow_single_file = True,
            cfg = "exec",
        ),
        "zig_runtime": attr.label(
            mandatory = True,
            cfg = "exec",
        ),
        "toolchain_file": attr.label(
            mandatory = True,
            allow_single_file = True,
        ),
    },
)

def _select_output_group_impl(ctx):
    files = ctx.attr.target[OutputGroupInfo][ctx.attr.group]
    return [DefaultInfo(files = files)]

select_output_group = rule(
    implementation = _select_output_group_impl,
    attrs = {
        "target": attr.label(mandatory = True),
        "group": attr.string(mandatory = True),
    },
)
