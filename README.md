# 🧠 Tailnet Labs — *tailnet-node*

> *“It works on my machine™” — you, probably.*

---

## 🧩 Annotation

Welcome to the *beta* developer environment where chaos meets craftsmanship. This is **Tailnet Node**, the core project that pretends to blend everything — **AI**, **IoT**, **QEMU**, **LXC**, and **Go-powered wizardry** — into one barely-contained beast.

Here’s the deal: we run things *natively* when we can, we cheat with Docker when we must, and we use **Tailscale** because reinventing networking is a full-time job (and we already have one). Yes, it’s proprietary — yes, we know — and no, you can’t sue us for experimenting.

---

## 🧪 Status

**EXPERIMENTAL AF.**  ☢️  Expect enlightenment, despair, or both. If it breaks, you get to keep both pieces.

---

## 🧰 What This Repo Actually Does

* Spawns a Tailnet-aware environment via Caddy and shell scripts that barely apologize for their existence.
* Gives you a portable lab setup for experimenting with embedded Tailnet services.
* Keeps everything stupidly small so you can read it without scrolling for eternity.

Files that matter:

* 🐚 `init.sh` — the mad scientist’s entrypoint.
* 🧱 `compose.yml` — because Docker is the duct tape of modern devops.
* ☁️ `kub