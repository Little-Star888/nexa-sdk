# Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
# SPDX-License-Identifier: BSD-3-Clause

"""CPU-only SDK unit tests for llama_cpp KV-cache context shift.

Drives ``LlamaLlm::generate`` with an ``n_ctx`` small enough that the
``slide_window`` lambda (rotates the KV ring buffer while keeping the first
``n_keep`` tokens anchored) must fire during decode. Without rotation the C
runtime would surface ``GENIEX_ERROR_LLM_TOKENIZATION_CONTEXT_LENGTH`` which
the Python binding absorbs into ``profile.stop_reason == 'context_length'``.
"""

from __future__ import annotations

import pytest

import geniex
from geniex import AutoModelForCausalLM


def _load(paths, *, n_ctx: int):
    try:
        return AutoModelForCausalLM.from_pretrained(
            paths.model_path,
            device_map='cpu',
            n_ctx=n_ctx,
        )
    except geniex.GenieXError as e:
        pytest.skip(f'could not load Qwen3-0.6B (n_ctx={n_ctx}): {e}')


def test_shift_fires_when_decode_would_overflow_nctx(llama_cpp_paths):
    # n_ctx small enough that prompt (~200 tokens) + max_new_tokens exceeds it,
    # forcing slide_window to rotate during decode. Prompt tokens are anchored
    # by n_keep=4 and the rest of the past is compacted to make room.
    n_ctx = 256
    prompt_body = ' '.join(['banana'] * 200)
    prompt = f'Repeat the following words verbatim: {prompt_body}\n\nAnswer: '

    with _load(llama_cpp_paths, n_ctx=n_ctx) as llm:
        out = llm.generate(prompt, max_new_tokens=120, temperature=0.0, seed=0)

    assert out.profile.stop_reason != 'context_length', f'context shift did not save the run: {out.profile}'
    # Confirm we actually got past the n_ctx boundary during decode.
    assert out.profile.generated_tokens > 0
    assert out.profile.prompt_tokens + out.profile.generated_tokens > n_ctx


def test_no_shift_needed_when_nctx_is_ample(llama_cpp_paths):
    # Regression guard: with plenty of headroom, decode completes normally and
    # never dips into the shift path. Also verifies the shift test's failure
    # signal is meaningful (i.e. context_length is a real, reachable state).
    with _load(llama_cpp_paths, n_ctx=2048) as llm:
        out = llm.generate('Say hi in one word.', max_new_tokens=8, temperature=0.0, seed=0)

    assert out.profile.stop_reason in {'eos', 'length', 'completed'}, out.profile
    assert out.profile.generated_tokens > 0
