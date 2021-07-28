##########################################################################
# Copyright 2021 NetApp, Inc. All Rights Reserved.
#
# CONFIDENTIALITY NOTICE:  THIS SOFTWARE CONTAINS CONFIDENTIAL INFORMATION OF
# NETAPP, INC. USE, DISCLOSURE OR REPRODUCTION IS PROHIBITED WITHOUT THE PRIOR
# EXPRESS WRITTEN PERMISSION OF NETAPP, INC.
##########################################################################
"""This module contains remote ssh connection and command execution methods"""

import logging
import sys

import paramiko

from com.data.credentials import HostCredentials


class SSHManager:
    """Class for interacting systems using ssh."""

    def __init__(self, credentials, port=22):
        self.port = 22
        self.credentials = credentials
        pass

    def run_cmd(self, cmd) -> str:
        """Returns console output for the command execution.
        :param cmd: comamnd to execute
        :type cmd: str
        :return: the console out put
        :rtype: str
        """

        i = 0
        logging.info("Connecting to %s " % self.credentials.address)
        try:
            ssh = paramiko.SSHClient()
            ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
            ssh.connect(self.credentials.address, self.port, self.credentials.user, self.credentials.rootPassword)
        except paramiko.AuthenticationException:
            logging.info("Authentication failed when connecting to %s" % self.credentials.address)
        except:
            logging.info("Could not connect to %s , may be network issue or ssh is disabled" % self.credentials.address)
        # execute commands
        stdin, stdout, stderr = ssh.exec_command(cmd)
        console_out = stdout.readlines()
        console_out = "".join(console_out)
        # logging.info("\nConsole Output for command: {} \n {}".format(cmd,console_out))
        # Close SSH connection
        ssh.close()
        return console_out


def main(args=None):
    if args is None:
        print("Arguments expected")
    else:
        # args = { <commands>}
        ssh = SSHManager(HostCredentials('harvest-1', '10.193.113.50', 'netapp', 'linux'))
        ssh.run_cmd(cmd=args[0])
    return


if __name__ == "__main__":
    main(sys.argv[1:])
