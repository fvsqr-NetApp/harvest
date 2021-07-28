##########################################################################
# Copyright 2021 NetApp, Inc. All Rights Reserved.
#
# CONFIDENTIALITY NOTICE:  THIS SOFTWARE CONTAINS CONFIDENTIAL INFORMATION OF
# NETAPP, INC. USE, DISCLOSURE OR REPRODUCTION IS PROHIBITED WITHOUT THE PRIOR
# EXPRESS WRITTEN PERMISSION OF NETAPP, INC.
##########################################################################
"""This  enum class represents all supported object types."""

from enum import Enum


class ObjectType(Enum):
    VOLUME = "volume"
    LUN = "lun"
    NODE = "node"
    QTREE = "qtree"
    AGGREGATE = "aggr"

    @staticmethod
    def values():
        return list(map(lambda ot: ot.value, ObjectType))


print(ObjectType.values())
