# Running Gemma-4-26B (Q4_0 QAT) on Hexagon NPU with MTP Speculative Decoding

This tutorial walks through building llama.cpp with the **GGML Hexagon (HTP)** backend on
**Windows on Snapdragon**, and running the **Gemma-4-26B Q4_0 QAT** model with **MTP
(Multi-Token Prediction) speculative decoding** for faster generation.

We run `llama-server` on the Snapdragon device and connect to it from any client machine
with `llama-cli`.

> **Prerequisites**
> This guide assumes the OpenCL SDK, Hexagon SDK, GPU/NPU drivers, and a signing
> certificate are already installed and that NPU test-signing is enabled. If not, follow
> [`docs/backend/snapdragon/windows.md`](docs/backend/snapdragon/windows.md) first — it
> covers driver installation, SDK setup, and creating/trusting the HTP signing certificate.

---

## 1. Build llama.cpp with the Hexagon backend

Open **PowerShell** in your llama.cpp checkout and set the environment variables that point
at the SDKs and your HTP signing certificate:

```powershell
PS C:\Users\HCKTest\src\llama.cpp> $env:OPENCL_SDK_ROOT="C:\Qualcomm\OpenCL_SDK\2.3.2"
PS C:\Users\HCKTest\src\llama.cpp> $env:HEXAGON_HTP_CERT="C:\Users\HCKTest\Certs\ggml-htp-v1.pfx"
PS C:\Users\HCKTest\src\llama.cpp> $env:HEXAGON_SDK_ROOT="C:\Qualcomm\Hexagon_SDK\6.6.0.0"
```

Configure the build using the Snapdragon release preset. We also enable the pre-built Adreno
OpenCL binary kernels for the GPU backend:

```powershell
PS C:\Users\HCKTest\src\llama.cpp> cmake --preset arm64-windows-snapdragon-release -B build-wos -DGGML_OPENCL_USE_ADRENO_BIN_KERNELS=ON
...
-- Including OpenCL backend
-- Including Hexagon backend
...
-- Build files have been written to: C:/Users/HCKTest/src/llama.cpp/build-wos
```

Build it:

```powershell
PS C:\Users\HCKTest\src\llama.cpp> cmake --build .\build-wos
...
[144/356] Performing build step for 'htp-v73'
...
-- Installing: .../libggml-htp-v73.so
-- Installing: .../libggml-htp-v75.so
-- Installing: .../libggml-htp-v79.so
-- Installing: .../libggml-htp-v81.so
```

Install into a self-contained package folder:

```powershell
PS C:\Users\HCKTest\src\llama.cpp> cmake --install .\build-wos --prefix .\pkg-snapdragon
-- Install configuration: "Release"
-- Installing: .../pkg-snapdragon/bin/llama-server.exe
-- Installing: .../pkg-snapdragon/bin/llama-cli.exe
-- Installing: .../pkg-snapdragon/lib/libggml-hexagon.so
-- Installing: .../pkg-snapdragon/lib/libggml-htp-v73.so
...
```

You now have a ready-to-run package under `pkg-snapdragon\` with all backends (CPU, Adreno
OpenCL, and Hexagon HTP) and the signed HTP ops libraries.

> **Tip:** verify the HTP ops `.cat` file is properly signed before running:
> ```powershell
> PS> signtool.exe verify /v /pa .\pkg-snapdragon\lib\libggml-htp.cat
> ```

---

## 2. Get the models

MTP speculative decoding uses **two GGUFs**:

| Role | File | Source |
|------|------|--------|
| Main / target model | `gemma-4-26B_q4_0-it.gguf` | [google/gemma-4-26B-A4B-it-qat-q4_0-gguf](https://huggingface.co/google/gemma-4-26B-A4B-it-qat-q4_0-gguf/blob/main/gemma-4-26B_q4_0-it.gguf) |
| Draft / MTP model | `gemma-4-26b-A4B-it-assistant-Q4_0-q4emb.gguf` | [RachidAR/gemma-4-26B-A4B-it-qat-assistant-q4_0-gguf](https://huggingface.co/RachidAR/gemma-4-26B-A4B-it-qat-assistant-q4_0-gguf/blob/main/gemma-4-26b-A4B-it-assistant-Q4_0-q4emb.gguf) |

The draft model proposes several tokens ahead using the Multi-Token Prediction heads, and
the main model verifies them in a single batch — accepted tokens are effectively "free",
which is what boosts the generation rate.

Download both files into a convenient folder, e.g. `..\gguf\` relative to the checkout:

```bash
$ mkdir -p ../gguf && cd ../gguf

# Main (target) model — Gemma-4-26B Q4_0 QAT
$ wget https://huggingface.co/google/gemma-4-26B-A4B-it-qat-q4_0-gguf/resolve/main/gemma-4-26B_q4_0-it.gguf

# Draft (MTP assistant) model
$ wget https://huggingface.co/RachidAR/gemma-4-26B-A4B-it-qat-assistant-q4_0-gguf/resolve/main/gemma-4-26b-A4B-it-assistant-Q4_0-q4emb.gguf
```

> **Note:** the `google/...` repo is gated — accept the license terms on the model page and
> authenticate (e.g. `huggingface-cli login`) before downloading, or use the
> `huggingface-cli download` / `hf download` CLI.

---

## 3. Start the server (from Git Bash)

The server is launched from a **Git Bash** shell so we can export the runtime environment
variables the Hexagon backend expects.

`ADSP_LIBRARY_PATH` tells the DSP loader where to find the signed HTP ops libraries, and
`GGML_HEXAGON_OPPOLL=1` enables busy-polling for op-batch completions (lower latency, at the
cost of a spinning CPU thread):

```bash
$ export ADSP_LIBRARY_PATH=$(pwd)/pkg-snapdragon/lib
$ export GGML_HEXAGON_OPPOLL=1

$ ./pkg-snapdragon/bin/llama-server --no-mmap \
    -m  ../gguf/gemma-4-26B_q4_0-it.gguf \
    -md ../gguf/gemma-4-26b-A4B-it-assistant-Q4_0-q4emb.gguf \
    --spec-type draft-mtp --spec-draft-n-max 3 \
    -fa on -ngl 99 \
    --ctx-size 8192 --ubatch-size 1024 -t 6 --poll 1000 \
    --device HTP0 --spec-draft-device HTP0 \
    --host 10.42.1.174 --port 8080
```

### What the flags do

| Flag | Meaning |
|------|---------|
| `--no-mmap` | Load weights into memory instead of mmap (recommended for HTP). |
| `-m` | Main (target) model. |
| `-md` | Draft model — here the MTP assistant. |
| `--spec-type draft-mtp` | Use Multi-Token Prediction heads for drafting. |
| `--spec-draft-n-max 3` | Draft up to 3 tokens per step. |
| `-fa on` | Enable Flash Attention (runs on the Hexagon NPU). |
| `-ngl 99` | Offload all layers to the NPU (HTP behaves like a "GPU" for `-ngl`). |
| `--ctx-size 8192` | Context window. |
| `--ubatch-size 1024` | Physical batch size for prompt processing. |
| `-t 6` | CPU worker threads. |
| `--poll 1000` | CPU poll level (busy-wait for host-side work). |
| `--device HTP0` | Run the main model on Hexagon session HTP0. |
| `--spec-draft-device HTP0` | Run the draft model on the same HTP session. |
| `--host` / `--port` | Bind address — use the device's LAN IP so remote clients can reach it. |

On startup you'll see the Hexagon backend allocate its session, e.g.:

```
ggml-hex: Hexagon backend (experimental) : allocating new registry : ndev 1
ggml-hex: Hexagon Arch version v81
ggml-hex: allocating new session: HTP0
load_tensors: offloaded 63/63 layers to GPU
```

Leave the server running.

---

## 4. Connect from a client device

From **any other machine on the same network**, point `llama-cli` at the server with
`--server-base` and feed it a prompt file:

```bash
$ llama-cli --server-base http://10.42.1.174:8080 -f ../sample_prompt_1024.txt
```

`--server-base` makes `llama-cli` act as a thin client — it does **not** load the model
locally; all compute happens on the Snapdragon server. When generation finishes the client
prints a one-line summary:

```
[ Prompt: 513.8 t/s | Generation: 25.5 t/s ]
```

---

## 5. Reading the server-side timings

While serving, the server logs per-slot timing. `tg` is the running generation rate and
`tg_3s` is the rate over the last 3 seconds:

```
I slot launch_slot_: id  3 | task 0 | processing task, is_child = 0
I slot print_timing: id  3 | task 0 | n_decoded =    101, tg =  28.69 t/s, tg_3s =  28.68 t/s
I slot print_timing: id  3 | task 0 | n_decoded =    198, tg =  29.89 t/s, tg_3s =  31.26 t/s
I slot print_timing: id  3 | task 0 | n_decoded =    270, tg =  27.99 t/s, tg_3s =  23.81 t/s
...
I slot print_timing: id  3 | task 0 | n_decoded =    791, tg =  25.48 t/s, tg_3s =  29.85 t/s
```

At the end of the request you get the full breakdown:

```
I slot print_timing: id  3 | task 0 | prompt eval time =    1442.27 ms /   741 tokens (    1.95 ms per token,   513.77 tokens per second)
I slot print_timing: id  3 | task 0 |        eval time =   31163.44 ms /   794 tokens (   39.25 ms per token,    25.48 tokens per second)
I slot print_timing: id  3 | task 0 |       total time =   32605.71 ms /  1535 tokens
I slot print_timing: id  3 | task 0 |    graphs reused =        275
I slot print_timing: id  3 | task 0 | draft acceptance = 0.61871 (  516 accepted /   834 generated), mean len =  2.86
I slot      release: id  3 | task 0 | stop processing: n_tokens = 1535, truncated = 0
```

The key MTP line is **`draft acceptance`**:

- **`0.61871`** — 61.9% of drafted tokens were accepted by the main model.
- **`516 accepted / 834 generated`** — out of 834 tokens the draft model proposed, 516 were
  accepted directly.
- **`mean len = 2.86`** — on average 2.86 tokens were confirmed per verification step.

Higher acceptance means more tokens verified per main-model pass, which directly translates
to a higher generation rate. If acceptance is low, try adjusting `--spec-draft-n-max` or
confirm you're using the matching MTP draft model for your target model.

---

## Troubleshooting

- **NPU device not found / library load errors** — make sure `ADSP_LIBRARY_PATH` points at
  the `pkg-snapdragon/lib` folder that contains the signed `libggml-htp-v*.so` and that the
  `libggml-htp.cat` signature verifies (`signtool.exe verify`). Test-signing must be enabled.
- **Verbose op logging** — set `GGML_HEXAGON_VERBOSE=1` on the server to see each op dispatched
  to the NPU.
- **More on speculative decoding** — see [`docs/speculative.md`](docs/speculative.md) for all
  `--spec-type` variants and tuning.
