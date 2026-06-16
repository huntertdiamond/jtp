# Acknowledgments

`jtp` is a fork of [`wtp` / Worktree Plus](https://github.com/satococoa/wtp).

Overwhelming credit belongs to the original `wtp` project and its author,
[Satoshi Ebisawa](https://github.com/satococoa). The upstream project provided
the foundation for this repository, including the CLI shape, configuration file
format, hook execution model, shell integration, test strategy, documentation
structure, and release packaging.

This fork adapts that work for Jujutsu (`jj`) workspace workflows. The goal is
to preserve the user experience of `wtp` while replacing the underlying Git
worktree operations with Jujutsu workspace and bookmark operations.

The original source is MIT licensed. The original copyright notice remains in
[`LICENSE`](LICENSE), and this fork should continue to preserve that notice in
accordance with the license.
