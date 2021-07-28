##########################################################################
# Copyright 2021 NetApp, Inc. All Rights Reserved.
#
# CONFIDENTIALITY NOTICE:  THIS SOFTWARE CONTAINS CONFIDENTIAL INFORMATION OF
# NETAPP, INC. USE, DISCLOSURE OR REPRODUCTION IS PROHIBITED WITHOUT THE PRIOR
# EXPRESS WRITTEN PERMISSION OF NETAPP, INC.
##########################################################################
"""This module contains custom pytest fixtures."""
import os

import pytest
import yaml

from com.data import credentials
from com.data.credentials import HostCredentials
from com.object_types import ObjectType

HARVEST_ROOT_DIR = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# Default tests root dir
TESTS_ROOT_DIR = os.path.join(HARVEST_ROOT_DIR, "tests")

# Default path of the test-params.yml file.
DEFAULT_TEST_PARAMS_PATH = os.path.join(TESTS_ROOT_DIR, "resources", "test_params.yml")

# Default path of the harvest config file.
DEFAULT_HARVEST_FILE_PATH = os.path.join(TESTS_ROOT_DIR, "resources", "harvest.yml")


##########################################################################
# Pytest Fixtures
# See: https://docs.pytest.org/en/stable/fixture.html
##########################################################################


########################################
# Test Parameter Fixtures
########################################
@pytest.fixture(scope="session")
def test_params_dict() -> dict:
    """Reads the  test params YAML file into a dictionary to be leveraged by the tests.
    :return: dictionary containing all  test params
    :rtype: dict
    """

    with open(DEFAULT_TEST_PARAMS_PATH) as test_params_json:
        return yaml.load(test_params_json, Loader=yaml.FullLoader)


@pytest.fixture(scope="session")
def get_host_data() -> HostCredentials:
    """Reads the test params data and returns the list of hosts.
    :type data: dict
    :return: harvest host credentials
    :rtype: credentials
    """
    with open(DEFAULT_TEST_PARAMS_PATH) as test_params_json:
        data = yaml.load(test_params_json, Loader=yaml.FullLoader)
        host_data = data['Hosts'][0]
        assert isinstance(host_data, object)
        return HostCredentials(host_data['host_name'], host_data['address'], host_data['root_password'],
                               host_data['os_type'])


@pytest.fixture(scope="session")
def get_object_types() -> list:
    """returns all supported object types.
    :return: list of object types
    :rtype: list
    """
    return ObjectType.values()


@pytest.fixture(scope="session")
def get_harvest_file_path() -> str:
    """returns default base harvest config file path.
    :return: harvest config file path credentials
    :rtype: credentials
    """
    return DEFAULT_HARVEST_FILE_PATH
