##########################################################################
# Copyright 2021 NetApp, Inc. All Rights Reserved.
#
# CONFIDENTIALITY NOTICE:  THIS SOFTWARE CONTAINS CONFIDENTIAL INFORMATION OF
# NETAPP, INC. USE, DISCLOSURE OR REPRODUCTION IS PROHIBITED WITHOUT THE PRIOR
# EXPRESS WRITTEN PERMISSION OF NETAPP, INC.
##########################################################################
"""This module contains logging related configuration and util methods"""

import logging


class Logger(object):
    """Class for interacting and configuring the the  logger."""

    @staticmethod
    def getLogger(module_name: str) -> logging.Logger:
        """Returns an instance of the logger.
        :param name: the name of the module
        :type name: str
        :return: the logger object
        :rtype: logging.Logger
        """

        if module_name is None:
            module_name = "harvest"
        logger = logging.getLogger(module_name)
        logger.setLevel(logging.INFO)
        handler = logging.StreamHandler()
        handler.setLevel(logging.INFO)
        formatter = logging.Formatter('[%(asctime)s]  %(levelname)s - {%(filename)s:%(lineno)d} - %(message)s',
                                      '%m-%d %H:%M:%S')
        handler.setFormatter(formatter)
        if not logger.handlers:
            logger.addHandler(handler)
        return logger


class StepLogger:
    """Class for creating a step logging"""

    def __init__(self, logger):
        """
        :type logger: object
        """
        self.step_count = 1
        self.logger = logger

    def step(self, step_msg: str) -> None:
        """Test case step log .
        :param step_msg: test step message
        """
        space = " " * 24
        newline = "\n"
        msg = f'Step {self.step_count}: {step_msg}'
        marker_str = '=' * len(msg)
        msg_str = f'{marker_str}{newline}{space}{msg}'
        self.step_count += 1

        # Prints the test step.
        self.logger.info(msg_str)
        self.logger.info(marker_str)
