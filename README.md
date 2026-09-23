# mokri-potok-portal

The deploy repo of the Mokri Potok village's portal. The portal itself is
**Porta Pagi** — `github.com/dennislapchenko/porta-pagi` — built by this
collective and run from here: this repo holds the village's compose file and
env, the image tag it runs, its map seed and its VM notes. `CLAUDE.md` says
what lives where and how a release rolls.

```sh
task roll -- sha-<commit>   # run this porta-pagi build on the VM (writes BE_TAG, commits, pushes)
task vm:logs                # backend logs on the VM
task vm:code -- "<house>"   # fresh invite link for a house, when nobody is logged in
task vm:map                 # import the map seed from deploy/map once
task vm:codex -- codex.json # import the adopted codex once
task vm:backdrop -- pic.jpg # the picture behind the gate
task vm:backup              # copy the latest SQLite backup here
```
