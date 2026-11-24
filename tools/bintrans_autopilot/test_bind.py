"""Marker object used only by the test helper.

Holding this object does not authenticate a reviewer and does not authorize a merge.
"""

from __future__ import annotations


class TestBind:
    pass


BIND = TestBind()
