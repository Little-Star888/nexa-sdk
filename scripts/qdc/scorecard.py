#!/usr/bin/env python3
# ---------------------------------------------------------------------
# Copyright (c) 2025 Qualcomm Technologies, Inc. and/or its subsidiaries.
# SPDX-License-Identifier: BSD-3-Clause
# ---------------------------------------------------------------------
"""Scorecard: collect per-model performance using the geniex Python API.

Usage:
    pip install geniex
    python scorecard.py --model-url <URL> --device <chipset>

Measures prefill speed, decode speed, and TTFT across cpu/gpu/npu
device maps at multiple context lengths.
"""

from __future__ import annotations

import argparse
import csv
import json
import logging
import os
import ssl
import sys
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path

import geniex

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s - %(message)s",
)
log = logging.getLogger(__name__)

CONTEXT_LENGTHS = [512, 1024, 4096]
DEVICE_MAPS = ["cpu", "gpu", "npu"]

MODELS_FILE = Path(__file__).parent / "scorecard-models.json"


@dataclass
class PerfSample:
    device_map: str
    ctx_len: int
    prompt_tokens: int
    generated_tokens: int
    prefill_tps: float
    decode_tps: float
    ttft_ms: int


@dataclass
class ScorecardResult:
    model_url: str
    chipset: str
    date: str
    samples: list[PerfSample] = field(default_factory=list)


def _generate_prompt(target_tokens: int) -> str:
    """Generate a prompt string of approximately target_tokens tokens.

    Approximation: ~4 chars per token for English text.
    """
    base = (
        "The following is a detailed discussion about artificial intelligence, "
        "machine learning, and the future of technology. "
    )
    target_chars = target_tokens * 4
    repeats = max(1, target_chars // len(base))
    return (base * repeats)[:target_chars]


def _download_model(url: str, dest: Path) -> None:
    if dest.exists() and dest.stat().st_size > 1024 * 1024:
        log.info("Model already cached at %s", dest)
        return
    dest.parent.mkdir(parents=True, exist_ok=True)
    log.info("Downloading model from %s ...", url)
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    urllib.request.urlretrieve(url, str(dest), context=ctx)
    log.info("Downloaded %.1f MB", dest.stat().st_size / 1e6)


def _is_vlm(url: str) -> bool:
    return "-VL-" in url.upper()


def run_scorecard(
    model_url: str,
    chipset: str,
    model_path: Path,
    mmproj_path: Path | None = None,
    n_predict: int = 128,
) -> ScorecardResult:
    """Run the scorecard benchmarks across all device maps and context lengths."""
    result = ScorecardResult(
        model_url=model_url,
        chipset=chipset,
        date=datetime.now().strftime("%Y-%m-%d"),
    )

    is_vlm = _is_vlm(model_url)

    for device_map in DEVICE_MAPS:
        log.info("=== Device: %s ===", device_map)
        try:
            if is_vlm and mmproj_path:
                model = geniex.AutoModelForVision2Seq.from_pretrained(
                    str(model_path),
                    device_map=device_map,
                    mmproj_path=str(mmproj_path),
                )
            else:
                model = geniex.AutoModelForCausalLM.from_pretrained(
                    str(model_path),
                    device_map=device_map,
                )
        except Exception as e:
            log.warning("Failed to load model with device_map=%s: %s", device_map, e)
            continue

        try:
            for ctx_len in CONTEXT_LENGTHS:
                log.info("  CTX=%d ...", ctx_len)
                prompt = _generate_prompt(ctx_len)
                try:
                    output = model.generate(
                        prompt,
                        max_new_tokens=n_predict,
                        temperature=0.0,
                        seed=1,
                    )
                    p = output.profile
                    sample = PerfSample(
                        device_map=device_map,
                        ctx_len=ctx_len,
                        prompt_tokens=p.prompt_tokens,
                        generated_tokens=p.generated_tokens,
                        prefill_tps=p.prefill_speed,
                        decode_tps=p.decode_speed,
                        ttft_ms=p.ttft,
                    )
                    result.samples.append(sample)
                    log.info(
                        "    prefill=%.1f t/s  decode=%.1f t/s  ttft=%d ms  (pp=%d, gen=%d)",
                        sample.prefill_tps,
                        sample.decode_tps,
                        sample.ttft_ms,
                        sample.prompt_tokens,
                        sample.generated_tokens,
                    )
                except Exception as e:
                    log.warning("    Failed CTX=%d: %s", ctx_len, e)
                model.reset()
        finally:
            model.close()

    return result


def write_results_yaml(result: ScorecardResult, output_path: Path) -> None:
    import yaml

    output_path.parent.mkdir(parents=True, exist_ok=True)
    perf: dict = {}
    for dm in DEVICE_MAPS:
        perf[dm] = {}
        for ctx in CONTEXT_LENGTHS:
            sample = next(
                (s for s in result.samples if s.device_map == dm and s.ctx_len == ctx),
                None,
            )
            perf[dm][f"ctx_{ctx}"] = {
                "prefill_tps": sample.prefill_tps if sample else None,
                "decode_tps": sample.decode_tps if sample else None,
                "ttft_ms": sample.ttft_ms if sample else None,
                "prompt_tokens": sample.prompt_tokens if sample else None,
                "generated_tokens": sample.generated_tokens if sample else None,
            }

    data = {
        "model": result.model_url,
        "chipset": result.chipset,
        "date": result.date,
        "performance": perf,
    }
    with open(output_path, "w") as f:
        yaml.dump(data, f, default_flow_style=False, sort_keys=False)
    log.info("Wrote YAML results to %s", output_path)


def write_results_csv(result: ScorecardResult, output_path: Path) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    model_name = Path(result.model_url).stem
    rows = []
    for s in result.samples:
        rows.append({
            "model": model_name,
            "chipset": result.chipset,
            "date": result.date,
            "device_map": s.device_map,
            "ctx_len": s.ctx_len,
            "prompt_tokens": s.prompt_tokens,
            "generated_tokens": s.generated_tokens,
            "prefill_tps": s.prefill_tps,
            "decode_tps": s.decode_tps,
            "ttft_ms": s.ttft_ms,
        })
    if not rows:
        return
    header = list(rows[0].keys())
    write_header = not output_path.exists() or output_path.stat().st_size == 0
    with open(output_path, "a", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=header)
        if write_header:
            writer.writeheader()
        writer.writerows(rows)
    log.info("Appended %d rows to %s", len(rows), output_path)


def write_github_summary(result: ScorecardResult) -> None:
    summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
    if not summary_path:
        return

    model_short = Path(result.model_url).stem
    lines = [
        f"## Scorecard: {model_short} ({result.chipset})",
        f"Date: {result.date}",
        "",
        "| Device | CTX | Prefill t/s | Decode t/s | TTFT ms | PP tokens | Gen tokens |",
        "|--------|-----|-------------|------------|---------|-----------|------------|",
    ]
    for s in result.samples:
        lines.append(
            f"| {s.device_map} | {s.ctx_len} | "
            f"{s.prefill_tps:.1f} | {s.decode_tps:.1f} | "
            f"{s.ttft_ms} | {s.prompt_tokens} | {s.generated_tokens} |"
        )
    lines.append("")

    with open(summary_path, "a") as f:
        f.write("\n".join(lines) + "\n")


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--model-url", required=True, help="Direct URL to the GGUF model")
    p.add_argument("--mmproj-url", default="", help="URL to the mmproj GGUF (VLM only)")
    p.add_argument("--device", required=True, help="Chipset name (e.g. SM8750)")
    p.add_argument("--results-dir", type=Path, default=None, help="Output directory")
    p.add_argument("--n-predict", type=int, default=128, help="Tokens to generate per run")
    p.add_argument(
        "--model-dir",
        type=Path,
        default=Path("/data/local/tmp/gguf"),
        help="Directory to cache downloaded models",
    )
    args = p.parse_args()

    model_path = args.model_dir / "model.gguf"
    _download_model(args.model_url, model_path)

    mmproj_path = None
    if args.mmproj_url:
        mmproj_path = args.model_dir / "mmproj.gguf"
        _download_model(args.mmproj_url, mmproj_path)

    geniex.init()
    try:
        result = run_scorecard(
            model_url=args.model_url,
            chipset=args.device,
            model_path=model_path,
            mmproj_path=mmproj_path,
            n_predict=args.n_predict,
        )
    finally:
        geniex.deinit()

    if not result.samples:
        log.error("No performance data collected")
        return 1

    write_github_summary(result)

    if args.results_dir:
        model_name = Path(args.model_url).stem.replace(".gguf", "")
        write_results_yaml(result, args.results_dir / model_name / "scorecard.yaml")
        write_results_csv(result, args.results_dir / "scorecard.csv")

    log.info("Scorecard complete: %d samples collected", len(result.samples))
    return 0


if __name__ == "__main__":
    sys.exit(main())
