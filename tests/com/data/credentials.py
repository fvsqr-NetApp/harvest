##########################################################################
# Copyright 2021 NetApp, Inc. All Rights Reserved.
#
# CONFIDENTIALITY NOTICE:  THIS SOFTWARE CONTAINS CONFIDENTIAL INFORMATION OF
# NETAPP, INC. USE, DISCLOSURE OR REPRODUCTION IS PROHIBITED WITHOUT THE PRIOR
# EXPRESS WRITTEN PERMISSION OF NETAPP, INC.
##########################################################################
"""This module contains harvest host credentials"""

from dataclasses import dataclass


@dataclass
class HostCredentials:
    """This  class represents harvest host credentials."""

    def __init__(self, host_name, address, root_password, os_type):
        self.hostName = host_name
        self.address = address
        self.user = "root"
        self.rootPassword = root_password
        self.os_type = os_type

    def __str__(self):
        return f'Host Details: name={self.hostName} , address={self.address},rootUser={self.user}, ' \
               f'rootPassword={self.rootPassword}, osType= {self.os_type} '

    def __repr__(self):
        return f'Host Details: name={self.hostName} , address={self.address},rootUser={self.user}, ' \
               f'rootPassword={self.rootPassword}, osType= {self.os_type}'
