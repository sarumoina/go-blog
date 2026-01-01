---
title: Formatting the disk
published on: 2026-01-01
updated on: 2026-01-01
category: vps
---

## Step 1: Create the mount point

    sudo mkdir /hdd

## Step 2: Edit `/etc/fstab`:

    sudo nano /etc/fstab
and the add the following:
    /dev/sdb1    /hdd    ext4    defaults    0    0

## Step 3A: Mount partition

    sudo mount /hdd

## Step 3B: For mounting existing drive

    sudo lsblk -f #take a note of the UUID
    sudo nano /etc/fstab
    #add this line in fstab
    UUID=abcd1234-5678-9ef0-1122-334455667788  /mnt/data  ext4  defaults  0  0 #change the UUID
    sudo mount -a

No errors = success ✅

