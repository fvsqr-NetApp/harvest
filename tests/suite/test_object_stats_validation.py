################################################################################
# Copyright 2021 NetApp, Inc. All Rights Reserved.
#
################################################################################
"""This module contains tests for validating object counter data  functionality."""
import pytest

from com.common.logger import Logger, StepLogger
from com.common.ssh_manager import SSHManager
from com.data.credentials import HostCredentials
from com.object_types import ObjectType


class TestObjectStatsValidation:
    "Tests to validate pollers end to end"

    @pytest.fixture(autouse=True)
    def store_resources_and_test_params(self, get_host_data: HostCredentials, get_harvest_file_path: str) -> None:
        """Stores the  test parameter dictionaries as part of the class.
        """
        self.hostData = get_host_data
        self.harvest_path = get_harvest_file_path
        self.sshManager = SSHManager(self.hostData)
        self.logger = Logger.getLogger("TestObjectMetricValidation")
        self.step_logger = StepLogger(self.logger)

    @pytest.mark.smoke
    @pytest.mark.counter
    @pytest.mark.parametrize("object_type",ObjectType.values())
    def test_object_counter_data(self,object_type):
        """
        Test to ensure pollers status is ok.
        """
        self.step_logger.step("Validating Poller Status")
        poller_statsu_cmd = "cd /opt/harvest;/opt/harvest/bin/harvest status"
        status_results = self.sshManager.run_cmd(poller_statsu_cmd)
        poller_info = status_results.splitlines()[2]
        prometheus_port = poller_info.strip().split()[6]

        # for object_type in get_object_types:
        # get counter data for object
        base_url_cmd = "curl -s http://localhost:{}/metrics | grep ^{}".format(prometheus_port, object_type)
        counter_results = self.sshManager.run_cmd(base_url_cmd)
        counter_data = counter_results.splitlines()[-1].split()[-1]
        self.logger.info("Counter Data for Object Type {} =>  {}".format(object_type,counter_data))
        assert float(counter_data) >= 0, "Invalid Data  {} for object type {} ".format(counter_data, object_type)
    # End of test test_object_counter_data



