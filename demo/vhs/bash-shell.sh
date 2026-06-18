#!/usr/bin/env bash
# Non-interactive bash for VHS demos — skip user rc files (mise prompts, etc.).
exec /bin/bash --norc --noprofile "$@"
