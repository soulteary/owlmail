# Control-store writer fence

Writable multiuser control stores hold an exclusive, non-blocking advisory lock
from before permission/schema changes until `Store.Close` finishes authority
operations and closes SQLite. The persistent `<control-database>.writer-lock`
sidecar is locked for the store lifetime. Never unlink or replace it to clear a
lock: identify and stop the owning process instead. A crash releases the OS lock,
so a leftover sidecar alone does not prevent restart.

The database's parent path is canonicalized and final symlinks are rejected for
both the database and fence. Both files must have exactly one hard link; this
prevents a different path from bypassing the fence. The fence is a regular 0600
file and is never unlinked on Close or failed initialization, avoiding distinct
inodes being locked under the same pathname. Keep parent directories private;
do not rename, unlink or replace a live database or fence.

Linux/macOS use `flock` on the sidecar; Windows uses `LockFileEx` on its first
byte. Locking the database inode itself was rejected after native macOS tests
produced SQLITE_BUSY during SQLite initialization. The separate file avoids
interfering with SQLite's own locking. Closing the descriptor or terminating the
process releases the fence. Failed initialization closes it, and repeated Close
calls are safe. No dependency or user-facing configuration is added.

This fence applies to upgraded code using writable `mailbox.Open`. Old binaries,
raw SQL tools and an administrator bypassing the Store do not honor it. Stop all
old writers before an upgrade or offline authority mutation. It is not a defense
against a hostile filesystem owner, nor certification of SQLite on NFS. Normal
standalone operation does not open a control store and remains unchanged.

`OpenReadOnly`/doctor inspection creates no lock file, changes no permissions and
may inspect an idle live authority. A consistent SQL read does **not** make live
mail files a consistent backup: exports and recovery still require a stopped
writer or a coordinated complete snapshot. The fence contains no authority data;
it can be excluded from offline backups and recreated on first writable open.
Never remove or restore it over an active process's inode.

The focused CI job executes second-writer rejection, hardlink refusal,
subprocess termination/reopen, read-only inspection and initialization failure
cleanup on Linux, macOS and Windows. The regular Linux race suite remains active;
cross-compilation is not treated as proof of Windows lock behavior.
