"""Cursor API errors without transport imports (launch package boundary)."""

from __future__ import annotations


class CursorApiError(Exception):
    def __init__(self, status: int, code: str, message: str):
        super().__init__(message)
        self.status = status
        self.code = code
        self.message = message

    @property
    def retryable(self) -> bool:
        return self.status >= 500 or self.status == 429 or self.status == 0
