# Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
# SPDX-License-Identifier: BSD-3-Clause

"""CPU-only SDK unit tests for VLM mtmd image-token placement.

Drives ``LlamaVlm::apply_chat_template`` end-to-end from Python. The plugin
consolidates every non-text content part in a message into ``N`` copies of
``mtmd_default_marker()`` (``<__media__>``) prepended to the concatenated text
of that message before feeding the result to the chat template.

The test asserts on the marker count and text presence — it does NOT run
``vlm.generate``, since that requires image decoding through mmproj and would
turn this into a device-scale integration test.
"""

from __future__ import annotations

from pathlib import Path

import pytest

import geniex
from geniex import AutoModelForVision2Seq

_MEDIA_MARKER = '<__media__>'

_REPO_ROOT = Path(__file__).resolve().parents[3]
_IMAGE_A = _REPO_ROOT / 'cli' / 'server' / 'docs' / 'ui' / 'favicon-32x32.png'
_IMAGE_B = _REPO_ROOT / 'tests' / 'assets' / 'quality_dog.jpg'


@pytest.fixture(scope='module')
def vlm(llama_cpp_vlm_paths):
    if not _IMAGE_A.is_file():
        pytest.skip(f'test image missing: {_IMAGE_A}')
    try:
        model = AutoModelForVision2Seq.from_pretrained(
            llama_cpp_vlm_paths.model_path,
            mmproj_path=llama_cpp_vlm_paths.mmproj_path,
            device_map='cpu',
        )
    except geniex.GenieXError as e:
        pytest.skip(f'could not load VLM: {e}')
    try:
        yield model
    finally:
        model.close()


@pytest.mark.parametrize('n_images', [0, 1, 2])
def test_media_markers_prepended_per_image(vlm, n_images):
    content: list[dict] = [
        {'type': 'image', 'image': str(_IMAGE_A)},
        {'type': 'image', 'image': str(_IMAGE_B) if _IMAGE_B.is_file() else str(_IMAGE_A)},
    ][:n_images]
    content.append({'type': 'text', 'text': 'describe the picture'})

    prompt = vlm.tokenizer.apply_chat_template([{'role': 'user', 'content': content}])
    assert prompt.count(_MEDIA_MARKER) == n_images
    assert 'describe the picture' in prompt


def test_text_only_message_has_no_media_marker(vlm):
    prompt = vlm.tokenizer.apply_chat_template([{'role': 'user', 'content': 'plain text, no image'}])
    assert _MEDIA_MARKER not in prompt
    assert 'plain text, no image' in prompt


def test_generate_rejects_missing_image_path(vlm):
    # apply_chat_template records that content referenced an image, then
    # generate() validates the images=[...] list against the filesystem.
    vlm.tokenizer.apply_chat_template(
        [
            {
                'role': 'user',
                'content': [
                    {'type': 'image', 'image': str(_IMAGE_A)},
                    {'type': 'text', 'text': 'hi'},
                ],
            }
        ]
    )
    with pytest.raises(FileNotFoundError):
        vlm.generate('irrelevant', images=[str(_IMAGE_A.parent / 'does-not-exist.png')])
