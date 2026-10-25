#!/usr/bin/env python3
"""Create, verify, and remove one ephemeral env file without following links."""

import os
import stat
import sys


def fail(message):
    print(message, file=sys.stderr)
    raise SystemExit(1)


def modes_enforced():
    return sys.platform.startswith("linux")


def is_inside(path, root):
    path = os.path.normcase(os.path.abspath(path))
    root = os.path.normcase(os.path.abspath(root))
    try:
        return os.path.commonpath([path, root]) == root
    except ValueError:
        return False


def is_redirect(path):
    if os.path.islink(path):
        return True
    if not sys.platform.startswith("win"):
        return False
    try:
        import ctypes

        get = ctypes.windll.kernel32.GetFileAttributesW
        get.argtypes = [ctypes.c_wchar_p]
        get.restype = ctypes.c_uint32
        attrs = get(str(path))
    except (AttributeError, OSError, ValueError):
        return False
    if attrs == 0xFFFFFFFF:
        return False
    return bool(attrs & 0x400)


def ancestor_allows_create(info):
    if not hasattr(os, "geteuid"):
        return True
    if info.st_uid == os.geteuid():
        return True
    mode = stat.S_IMODE(info.st_mode)
    sticky = stat.S_ISVTX | stat.S_IWOTH | stat.S_IXOTH
    return (mode & sticky) == sticky


def require_real_directory(path, label):
    try:
        info = os.lstat(path)
    except FileNotFoundError:
        fail(label + " is missing")
    if stat.S_ISLNK(info.st_mode) or is_redirect(path):
        fail("env path contains a symbolic link")
    if not stat.S_ISDIR(info.st_mode):
        fail("env directory is not a private directory")
    return info


def ensure_parent(path):
    path = os.path.abspath(path)
    if os.path.lexists(path):
        require_real_directory(path, "env directory")
        return
    grand = os.path.dirname(path)
    if grand == path:
        fail("env directory is missing")
    ensure_parent(grand)
    grand_info = require_real_directory(grand, "env directory")
    if not ancestor_allows_create(grand_info):
        fail("env directory is not a private directory")
    os.mkdir(path, 0o700)


def assert_lexical_outside(path, repo_root):
    candidate = os.path.abspath(path)
    for repo in (os.path.abspath(repo_root), os.path.realpath(repo_root)):
        if is_inside(candidate, repo):
            fail("env path is inside the repository")


def assert_outside_repo(path, repo_root):
    repo_values = [os.path.abspath(repo_root), os.path.realpath(repo_root)]
    parent = os.path.dirname(os.path.abspath(path))
    parent_real = os.path.realpath(parent)
    candidates = [os.path.abspath(path), os.path.join(parent_real, os.path.basename(path))]
    for candidate in candidates:
        for repo in repo_values:
            if is_inside(candidate, repo):
                fail("env path is inside the repository")


def assert_private_parent(parent):
    info = require_real_directory(parent, "env directory")
    if hasattr(os, "geteuid") and info.st_uid != os.geteuid():
        fail("env directory owner is not the current user")
    if modes_enforced():
        mode = stat.S_IMODE(info.st_mode)
        # A private directory can inherit the setgid bit. Group and other
        # must have no access. Clear the extra bits when we own that directory.
        if mode != 0o700 and (mode & 0o077) == 0 and hasattr(os, "geteuid") and info.st_uid == os.geteuid():
            os.chmod(parent, 0o700)
            info = os.lstat(parent)
            mode = stat.S_IMODE(info.st_mode)
        if mode != 0o700:
            fail("env directory permissions are not 0700 (%04o)" % mode)
    return info


def create_file(path, repo_root):
    if not os.path.isabs(path):
        fail("env path must be absolute")
    path = os.path.abspath(path)
    parent = os.path.dirname(path)
    name = os.path.basename(path)
    if not name or name in (".", ".."):
        fail("env path must be a file path")

    # Reject every existing symlink before creating anything.
    cursor = parent
    seen = []
    while True:
        seen.append(cursor)
        parent_of = os.path.dirname(cursor)
        if parent_of == cursor:
            break
        if not os.path.lexists(cursor):
            cursor = parent_of
            continue
        info = os.lstat(cursor)
        if stat.S_ISLNK(info.st_mode) or is_redirect(cursor):
            fail("env path contains a symbolic link")
        if not stat.S_ISDIR(info.st_mode):
            fail("env directory is not a private directory")
        cursor = parent_of
        if cursor in seen:
            break

    try:
        leaf = os.lstat(path)
    except FileNotFoundError:
        leaf = None
    if leaf is not None:
        if stat.S_ISLNK(leaf.st_mode) or is_redirect(path):
            fail("env path contains a symbolic link")
        fail("env file already exists")

    assert_lexical_outside(path, repo_root)
    ensure_parent(parent)
    assert_private_parent(parent)
    assert_outside_repo(path, repo_root)

    flags = os.O_CREAT | os.O_EXCL | os.O_WRONLY
    if modes_enforced():
        if not hasattr(os, "O_NOFOLLOW"):
            fail("O_NOFOLLOW is unavailable")
        flags |= os.O_NOFOLLOW
    elif hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    if hasattr(os, "O_CLOEXEC"):
        flags |= os.O_CLOEXEC

    previous = os.umask(0o077)
    fd = None
    try:
        fd = os.open(path, flags, 0o600)
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode):
            fail("env file is not a regular file")
        try:
            os.fchmod(fd, 0o600)
        except (AttributeError, OSError):
            if modes_enforced():
                fail("env file permissions are not 0600")
        info = os.fstat(fd)
        if modes_enforced() and stat.S_IMODE(info.st_mode) != 0o600:
            fail("env file permissions are not 0600")
        payload = sys.stdin.buffer.read()
        if not payload:
            fail("ephemeral env was empty")
        view = memoryview(payload)
        while view:
            written = os.write(fd, view)
            view = view[written:]
        os.fsync(fd)
        dev = info.st_dev
        ino = info.st_ino
        mode = stat.S_IMODE(info.st_mode)
    except SystemExit:
        if fd is not None:
            os.close(fd)
            fd = None
            discard_created(path)
        raise
    except Exception as exc:
        if fd is not None:
            os.close(fd)
            fd = None
            discard_created(path)
        fail("env file was not created: " + exc.__class__.__name__)
    else:
        os.close(fd)
    finally:
        os.umask(previous)
    print("ENV_DEV=%s" % dev)
    print("ENV_INO=%s" % ino)
    print("ENV_MODE=%04o" % mode)


def discard_created(path):
    try:
        info = os.lstat(path)
    except FileNotFoundError:
        return
    if stat.S_ISLNK(info.st_mode) or not stat.S_ISREG(info.st_mode):
        return
    try:
        os.unlink(path)
    except OSError:
        pass


def matching_identity(path, dev, ino):
    if not os.path.isabs(path):
        fail("env path must be absolute")
    flags = os.O_RDONLY
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    if hasattr(os, "O_CLOEXEC"):
        flags |= os.O_CLOEXEC
    try:
        info = os.lstat(path)
    except FileNotFoundError:
        fail("env file identity changed")
    if stat.S_ISLNK(info.st_mode) or is_redirect(path) or not stat.S_ISREG(info.st_mode):
        fail("env file identity changed")
    try:
        fd = os.open(path, flags)
    except OSError:
        fail("env file identity changed")
    try:
        current = os.fstat(fd)
    finally:
        os.close(fd)
    if current.st_dev != dev or current.st_ino != ino:
        fail("env file identity changed")
    if not stat.S_ISREG(current.st_mode):
        fail("env file identity changed")
    return current


def parse_identity(dev_text, ino_text):
    if not dev_text.isdigit() or not ino_text.isdigit():
        fail("env file identity is not numeric")
    return int(dev_text), int(ino_text)


def main():
    if len(sys.argv) < 2:
        fail("usage: private-env.py create|verify|remove ...")
    action = sys.argv[1]
    if action == "create":
        if len(sys.argv) != 4:
            fail("usage: private-env.py create PATH REPO_ROOT")
        create_file(sys.argv[2], sys.argv[3])
        return
    if action in ("verify", "remove"):
        if len(sys.argv) != 5:
            fail("usage: private-env.py " + action + " PATH DEV INO")
        dev, ino = parse_identity(sys.argv[3], sys.argv[4])
        matching_identity(os.path.abspath(sys.argv[2]), dev, ino)
        if action == "remove":
            info = os.lstat(os.path.abspath(sys.argv[2]))
            if stat.S_ISLNK(info.st_mode) or is_redirect(os.path.abspath(sys.argv[2])) or info.st_dev != dev or info.st_ino != ino:
                fail("env file identity changed")
            os.unlink(os.path.abspath(sys.argv[2]))
        return
    fail("unknown private-env action")


if __name__ == "__main__":
    main()
