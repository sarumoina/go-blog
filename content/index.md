---
title: Welcome to My Docs
published on: 2025-01-01
updated on: 2025-01-05
category: General
---

## Install GUI in Ubuntu 

**Install Minimal GUI (XFCE Core Only)**

```
# Update base system
sudo apt update && sudo apt upgrade -y

# Install XRDP + Xorg backend
sudo apt install -y xrdp xorgxrdp dbus-x11

# Install ultra-light MATE (no extras)
sudo apt install -y mate-core mate-session-manager

# Optional: basic utilities (recommended but still light)
sudo apt install -y mate-terminal caja marco

# Set MATE as default session for XRDP
echo "mate-session" > ~/.xsession
chmod +x ~/.xsession

# Allow XRDP user access to SSL cert
sudo adduser xrdp ssl-cert

# Ensure XRDP uses Xorg
sudo sed -i 's/^test -x/#test -x/' /etc/xrdp/startwm.sh
sudo sed -i 's/^exec .*/exec mate-session/' /etc/xrdp/startwm.sh

# Enable and restart XRDP
sudo systemctl enable xrdp
sudo systemctl restart xrdp

# Open RDP port (if firewall is enabled)
sudo ufw allow 3389/tcp

# Reboot recommended
sudo reboot
```

**To Remote GUI Access from Windows**

```
sudo apt install xrdp -y
sudo systemctl enable --now xrdp
```

### Creating a user

```
sudo addsuer myuser
sudo usermod -aG sudo myuser
```

### Deleting a user

```
sudo deluser username #or
sudo deluser --remove-home username
```

## Creating SSH keys

### Generate the key

```
ssh-keygen -t ed25519 -C "your_email@example.com"  #ed25519 is the newest encryption
```

Edit the config file

Go to `~/.ssh/config` and open it. (if not present, then create it)

```
Host github.com
        User git
        Hostname github.com
        PreferredAuthentications publickey
        IdentityFile ~/.ssh/github

Host anyname
        User root
        Hostname your_host_name
        PreferredAuthentications publickey
        IdentityFile ~/.ssh/your_private_key
```
### Copy the id to to remote:

```
ssh-copy-id -i ~/.ssh/key user@remote
```