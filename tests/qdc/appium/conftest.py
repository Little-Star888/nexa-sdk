# Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
# SPDX-License-Identifier: BSD-3-Clause

"""Appium session fixture — the session is what unlocks adb to the phone.
`test_run_suite` drives the device; JUnit XML is pushed to QDC_logs/results
so `run_qdc_pytest.py` collects it.
"""

import os

import pytest
from appium import webdriver
from utils import options, write_qdc_log

DEVICE_JUNIT = 'device-results.xml'


@pytest.fixture(scope='session', autouse=True)
def driver():
    return webdriver.Remote(command_executor='http://127.0.0.1:4723/wd/hub', options=options)


def pytest_sessionfinish(session, exitstatus):
    if os.path.exists(DEVICE_JUNIT):
        with open(DEVICE_JUNIT) as f:
            write_qdc_log('results/device-results.xml', f.read())
    # Always ship the harness log so a failed build/deploy/test is diagnosable.
    if os.path.exists('harness.log'):
        with open('harness.log') as f:
            write_qdc_log('harness.log', f.read())
