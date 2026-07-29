# Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
# SPDX-License-Identifier: BSD-3-Clause

"""llama_cpp LLM matrix: cpu, npu, gpu."""

from __future__ import annotations

import geniex
import pytest

from _models import LLAMA_CPP_LLM_MODEL, LLAMA_CPP_LLM_PRECISION
from _quality_data import (
    LLM_QUALITY_MAX_NEW_TOKENS,
    LLM_QUALITY_PROMPTS,
    LLM_QUALITY_SEED,
    LLM_QUALITY_TEMPERATURE,
)

pytestmark = pytest.mark.llm


@pytest.mark.parametrize('device_map', ['cpu', 'npu', 'gpu'])
def test_generate_blocking(llama_cpp_llm_paths, device_map):
    with geniex.AutoModelForCausalLM.from_pretrained(
        LLAMA_CPP_LLM_MODEL,
        precision=LLAMA_CPP_LLM_PRECISION,
        device_map=device_map,
    ) as llm:
        assert isinstance(llm, geniex.GenieXLLM)
        out = llm.generate('Say hi.', max_new_tokens=8, temperature=0.0, seed=42)
        assert isinstance(out, geniex.GenerateOutput)
        assert out.text
        assert out.profile.generated_tokens > 0


@pytest.mark.parametrize('device_map', ['cpu'])
def test_generate_stream(llama_cpp_llm_paths, device_map):
    with geniex.AutoModelForCausalLM.from_pretrained(
        LLAMA_CPP_LLM_MODEL,
        precision=LLAMA_CPP_LLM_PRECISION,
        device_map=device_map,
    ) as llm:
        streamer = llm.generate('Say hi.', max_new_tokens=8, temperature=0.0, seed=42, stream=True)
        assert isinstance(streamer, geniex.TextIteratorStreamer)
        chunks = list(streamer)
        assert chunks
        assert streamer.output is not None
        assert streamer.output.text


@pytest.mark.parametrize('device_map', ['cpu', 'npu', 'gpu'])
@pytest.mark.parametrize(('prompt', 'expected'), LLM_QUALITY_PROMPTS)
def test_quality_keywords(llama_cpp_llm_paths, device_map, prompt, expected):
    # Mirrors run_scorecard_posix.py:_section_quality_checks (test-llama.cpp):
    # same prompts, n_predict=256, seed=1, plugin-default sampler, chat
    # template applied. Upstream's `llama-completion` runs without `-no-cnv`,
    # so COMMON_CONVERSATION_MODE_AUTO wraps the prompt in the model's chat
    # template; feeding the raw string instead lets Qwen3-style models continue
    # in completion mode and the keyword only appears by sampler luck.
    with geniex.AutoModelForCausalLM.from_pretrained(
        LLAMA_CPP_LLM_MODEL,
        precision=LLAMA_CPP_LLM_PRECISION,
        device_map=device_map,
    ) as llm:
        formatted = llm.tokenizer.apply_chat_template(
            [{'role': 'user', 'content': prompt}],
            tokenize=False,
            add_generation_prompt=True,
        )
        out = llm.generate(
            formatted,
            max_new_tokens=LLM_QUALITY_MAX_NEW_TOKENS,
            temperature=LLM_QUALITY_TEMPERATURE,
            seed=LLM_QUALITY_SEED,
        )
        assert isinstance(out, geniex.GenerateOutput)
        assert out.text, f'empty completion for prompt={prompt!r}'
        # Hoist the comparison into a local bool so pytest's assertion
        # introspection has nothing to walk — otherwise out.text gets
        # echoed 4–5x per failure (lower() chain + GenerateOutput repr).
        matched = expected.lower() in out.text.lower()
        assert matched, (
            f'prompt={prompt!r} expected_substring={expected!r} ' f'device_map={device_map!r} got={out.text!r}'
        )


def _run_multi_turn(llm) -> list[str]:
    # Two-turn conversation: turn 1 introduces "Alice", turn 2 asks the model to
    # recall the name. Because the tokenizer/chat-template path is applied on
    # every turn against the growing history, this exercises the plugin's
    # prefix-match + KV-cache rollback, the sampler carrying across turns, and
    # (when spec is enabled) the speculative context tracking the target's
    # per-turn n_past.
    history: list[dict] = [{'role': 'system', 'content': 'Answer in one short sentence.'}]
    replies: list[str] = []
    for user in [
        'My name is Alice and I am 30 years old. Just acknowledge.',
        'What is my name?',
    ]:
        history.append({'role': 'user', 'content': user})
        prompt = llm.tokenizer.apply_chat_template(
            history,
            tokenize=False,
            add_generation_prompt=True,
            enable_thinking=False,
        )
        out = llm.generate(prompt, max_new_tokens=64, temperature=0.0, seed=42)
        assert isinstance(out, geniex.GenerateOutput)
        assert out.text, f'empty completion at turn {len(replies) + 1}'
        assert out.profile.generated_tokens > 0
        replies.append(out.text)
        history.append({'role': 'assistant', 'content': out.text})
    return replies


@pytest.mark.parametrize('device_map', ['cpu', 'npu', 'gpu'])
def test_multi_turn_recalls_prior_turn(llama_cpp_llm_paths, device_map):
    with geniex.AutoModelForCausalLM.from_pretrained(
        LLAMA_CPP_LLM_MODEL,
        precision=LLAMA_CPP_LLM_PRECISION,
        device_map=device_map,
    ) as llm:
        replies = _run_multi_turn(llm)
    assert 'alice' in replies[-1].lower(), (
        f'device_map={device_map!r} expected reply to recall "Alice", got={replies[-1]!r}'
    )


@pytest.mark.parametrize('device_map', ['cpu', 'npu', 'gpu'])
def test_multi_turn_with_ngram_simple(llama_cpp_llm_paths, device_map):
    # ngram-simple is self-speculative (no draft model), so it can reuse the
    # LLM fixture and works with any target. Exercises the setup_speculative /
    # decode_speculative code paths without needing an MTP-trained model.
    with geniex.AutoModelForCausalLM.from_pretrained(
        LLAMA_CPP_LLM_MODEL,
        precision=LLAMA_CPP_LLM_PRECISION,
        device_map=device_map,
        spec_type='ngram-simple',
        spec_n_max=3,
    ) as llm:
        replies = _run_multi_turn(llm)
    assert 'alice' in replies[-1].lower(), (
        f'device_map={device_map!r} expected reply to recall "Alice", got={replies[-1]!r}'
    )


@pytest.mark.parametrize('device_map', ['npu'])
def test_multi_turn_with_draft_mtp(llama_cpp_mtp_paths, device_map):
    # draft-mtp needs a target that ships MTP heads plus a matching assistant
    # draft; today only gemma-4 A4B fits, so this is NPU-only (the target is
    # ~15GB and only worth loading on the HTP backend). Loads via absolute
    # paths from the fixture to keep the AutoModelForCausalLM factory out of
    # the VLM branch that catalogue-stored mmproj would otherwise trigger.
    with geniex.AutoModelForCausalLM.from_pretrained(
        llama_cpp_mtp_paths['target'].model_path,
        device_map=f'llama_cpp:{device_map}',
        spec_type='draft-mtp',
        spec_draft_model=llama_cpp_mtp_paths['draft'].model_path,
        spec_n_max=3,
    ) as llm:
        assert isinstance(llm, geniex.GenieXLLM)
        replies = _run_multi_turn(llm)
    assert 'alice' in replies[-1].lower(), (
        f'device_map={device_map!r} expected reply to recall "Alice", got={replies[-1]!r}'
    )
