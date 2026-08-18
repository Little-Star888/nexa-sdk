# Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
# SPDX-License-Identifier: BSD-3-Clause

"""CPU-only SDK unit tests for LLM chat-template assembly.

Exercises ``LlamaLlm::apply_chat_template`` end-to-end from Python:

* the baked-in Qwen3 ChatML template (roles, sentinels, generation prompt)
* ``enable_thinking`` handling on a thinking-capable model
* ``tools`` accepted as ``list[dict]`` and pre-serialised JSON string
* ``chat_template_content`` load-time jinja override
"""

from __future__ import annotations

import json

import pytest

import geniex
from geniex import AutoModelForCausalLM


@pytest.fixture(scope='module')
def llm(llama_cpp_paths):
    try:
        model = AutoModelForCausalLM.from_pretrained(
            llama_cpp_paths.model_path,
            device_map='cpu',
        )
    except geniex.GenieXError as e:
        pytest.skip(f'could not load Qwen3-0.6B: {e}')
    try:
        yield model
    finally:
        model.close()


def test_baked_template_roles_and_sentinels(llm):
    prompt = llm.tokenizer.apply_chat_template(
        [
            {'role': 'system', 'content': 'You are a helpful assistant.'},
            {'role': 'user', 'content': 'hi'},
        ]
    )
    assert '<|im_start|>system' in prompt
    assert '<|im_start|>user' in prompt
    assert prompt.rstrip().endswith('<|im_start|>assistant')
    assert prompt.index('<|im_start|>system') < prompt.index('<|im_start|>user')


def test_enable_thinking_flag_reaches_template(llm):
    msgs = [{'role': 'user', 'content': 'hi'}]
    with_think = llm.tokenizer.apply_chat_template(msgs, enable_thinking=True)
    without_think = llm.tokenizer.apply_chat_template(msgs, enable_thinking=False)
    default_think = llm.tokenizer.apply_chat_template(msgs)  # None → auto-resolve to True
    assert default_think == with_think
    assert with_think != without_think


def test_tools_list_and_json_string_are_equivalent(llm):
    tool = {
        'type': 'function',
        'function': {
            'name': 'get_weather',
            'description': 'Get current weather.',
            'parameters': {
                'type': 'object',
                'properties': {'city': {'type': 'string'}},
                'required': ['city'],
            },
        },
    }
    msgs = [{'role': 'user', 'content': "what's the weather in Paris?"}]
    from_list = llm.tokenizer.apply_chat_template(msgs, tools=[tool])
    from_str = llm.tokenizer.apply_chat_template(msgs, tools=json.dumps([tool]))
    assert from_list == from_str
    assert 'get_weather' in from_list


def test_chat_template_content_load_override(llama_cpp_paths):
    jinja = (
        '{% for m in messages %}[{{ m.role }}] {{ m.content }}\n{% endfor %}'
        '{% if add_generation_prompt %}[assistant] {% endif %}'
    )
    try:
        model = AutoModelForCausalLM.from_pretrained(
            llama_cpp_paths.model_path,
            device_map='cpu',
            chat_template_content=jinja,
        )
    except geniex.GenieXError as e:
        pytest.skip(f'could not load with chat_template override: {e}')
    try:
        prompt = model.tokenizer.apply_chat_template(
            [
                {'role': 'system', 'content': 'be terse'},
                {'role': 'user', 'content': 'hello'},
            ]
        )
    finally:
        model.close()
    assert prompt == '[system] be terse\n[user] hello\n[assistant] '
