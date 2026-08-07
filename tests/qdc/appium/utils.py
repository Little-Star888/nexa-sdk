# Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
# SPDX-License-Identifier: BSD-3-Clause

"""adb + appium helpers for the QDC Android pytest run."""

from __future__ import annotations

import os
import subprocess
import tempfile

from appium.options.common import AppiumOptions

QDC_LOGS_PATH = '/data/local/tmp/QDC_logs'

options = AppiumOptions()
options.set_capability('automationName', 'UiAutomator2')
options.set_capability('platformName', 'Android')
options.set_capability('deviceName', os.getenv('ANDROID_DEVICE_VERSION'))


def write_qdc_log(filename: str, content: str) -> None:
    """Push content to /data/local/tmp/QDC_logs/<filename> for QDC collection."""
    dest = f'{QDC_LOGS_PATH}/{filename}'
    subprocess.run(['adb', 'shell', f'mkdir -p {os.path.dirname(dest)}'], check=False)
    with tempfile.NamedTemporaryFile(mode='w', suffix='.tmp', delete=False) as f:
        f.write(content)
        tmp = f.name
    try:
        subprocess.run(['adb', 'push', tmp, dest], stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    finally:
        os.unlink(tmp)
