## GenieX CLI

Command-line interface for running AI models locally on **Qualcomm** chipsets. Interfaces with the GenieX core runtime and supports two inference backends: **QAIRT** and **llama.cpp**.

### Logging

`GENIEX_LOG` controls log output across the CLI, the C/C++ SDK, and all language bindings (Go, Python, Android):

| Value   | Emits                                    |
|---------|------------------------------------------|
| `none`  | nothing                                  |
| `error` | errors only                              |
| `warn`  | warnings + errors                        |
| `info`  | info + warnings + errors (**default**)   |
| `debug` | debug + info + warnings + errors         |
| `trace` | everything (requires a debug build)      |

```bash
export GENIEX_LOG="debug"          # bash / zsh
$env:GENIEX_LOG="debug"            # PowerShell
```

`NO_COLOR=1` disables ANSI colors.

### Model pull

Pull a model non-interactively:

```bash
geniex pull <model>[:<precision>] --model-type <model-type>
```

Pull from a specific model hub:

```bash
geniex pull <model>
geniex pull <model> --model-hub aihub   # options: aihub, hf, localfs
```

Import a model from the local filesystem:

```bash
# hf download <model> --local-dir /path/to/modeldir
geniex pull <model> --model-hub localfs --local-path /path/to/modeldir
```
