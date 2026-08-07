# Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
# SPDX-License-Identifier: BSD-3-Clause

"""Host-side appium entry: deploy, run pytest on-device, pull JUnit back."""

from pathlib import Path

import deploy

PAYLOAD = Path(__file__).parents[1] / 'payload'
DEVICE_JUNIT = f'{deploy.DEV_TESTS}/device-results.xml'
HOST_JUNIT = 'device-results.xml'

PYTEST_ARGS = ['-m', 'not api', '--junitxml=device-results.xml']


def test_run_suite():
    deploy.deploy(PAYLOAD)
    rc = deploy.run_pytest(PYTEST_ARGS)
    deploy.adb('pull', DEVICE_JUNIT, HOST_JUNIT, check=False)
    assert Path(HOST_JUNIT).exists(), 'device produced no JUnit XML'
    assert rc == 0, f'on-device pytest exited {rc}'
