# Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Unit tests for the ensure_cached default-precision resolution (#1098).

When precision=None, ensure_cached must call query() to pick the hub's
default precision and pass it to pull() — so only one variant is downloaded.
"""

from __future__ import annotations

import geniex.model_manager as mm
from geniex._ffi._api import GenieXError
from geniex.model_manager import ModelPaths, ModelQuery, PrecisionCandidate


def _make_query(*precisions: tuple[str, bool]) -> ModelQuery:
    """Build a ModelQuery with the given (precision, is_default) pairs."""
    return ModelQuery(
        model_name='org/repo',
        runtime='llama_cpp',
        model_type='llm',
        candidates=[PrecisionCandidate(precision=p, size=0, is_default=d) for p, d in precisions],
    )


def _fake_paths() -> ModelPaths:
    return ModelPaths(
        model_path='/cache/org/repo/model.gguf',
        model_dir='/cache/org/repo',
        model_name='org/repo',
        runtime='llama_cpp',
        model_type='llm',
    )


def test_ensure_cached_resolves_default_precision(monkeypatch):
    """pull() receives the is_default precision when none is specified."""
    pulled = {}

    monkeypatch.setattr(mm, 'query', lambda name, **_kw: _make_query(('Q8_0', False), ('Q4_0', True)))
    monkeypatch.setattr(mm, 'pull', lambda name, *, precision=None, **_kw: pulled.update(precision=precision))
    monkeypatch.setattr(mm, 'get_paths', lambda _key: _fake_paths())
    monkeypatch.setattr(mm, 'resolve_alias', lambda name: name)
    monkeypatch.setattr(mm, '_ensure_init', lambda: None)

    mm.ensure_cached('org/repo')

    assert pulled['precision'] == 'Q4_0'


def test_ensure_cached_explicit_precision_skips_query(monkeypatch):
    """An explicit precision bypasses query() entirely."""
    queried = []

    monkeypatch.setattr(mm, 'query', lambda *_a, **_kw: queried.append(True) or _make_query(('Q4_0', True)))
    monkeypatch.setattr(mm, 'pull', lambda *_a, **_kw: None)
    monkeypatch.setattr(mm, 'get_paths', lambda _key: _fake_paths())
    monkeypatch.setattr(mm, 'resolve_alias', lambda name: name)
    monkeypatch.setattr(mm, '_ensure_init', lambda: None)

    mm.ensure_cached('org/repo', precision='Q8_0')

    assert queried == [], 'query() should not be called when precision is explicit'


def test_ensure_cached_local_path_skips_query(monkeypatch):
    """A local_path pull bypasses query() — the manifest is already on disk."""
    queried = []

    monkeypatch.setattr(mm, 'query', lambda *_a, **_kw: queried.append(True) or _make_query(('Q4_0', True)))
    monkeypatch.setattr(mm, 'pull', lambda *_a, **_kw: None)
    monkeypatch.setattr(mm, 'get_paths', lambda _key: _fake_paths())
    monkeypatch.setattr(mm, 'resolve_alias', lambda name: name)
    monkeypatch.setattr(mm, '_ensure_init', lambda: None)

    mm.ensure_cached('org/repo', local_path='/some/path')

    assert queried == []


def test_ensure_cached_query_failure_falls_through(monkeypatch):
    """A query() GenieXError is swallowed and pull() is still called."""
    pulled = {}

    monkeypatch.setattr(mm, 'query', lambda *_a, **_kw: (_ for _ in ()).throw(GenieXError(-1, 'offline')))
    monkeypatch.setattr(mm, 'pull', lambda name, *, precision=None, **_kw: pulled.update(precision=precision))
    monkeypatch.setattr(mm, 'get_paths', lambda _key: _fake_paths())
    monkeypatch.setattr(mm, 'resolve_alias', lambda name: name)
    monkeypatch.setattr(mm, '_ensure_init', lambda: None)

    mm.ensure_cached('org/repo')

    # precision stays None — pull decides on its own
    assert pulled['precision'] is None
