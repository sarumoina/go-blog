---
title: Mounting the disk
published on: 2026-01-03
updated on: 2026-01-03
category: vps
weight: 2
---

## Empty secondary storage

### Create the mount point:

    sudo mkdir /hdd

### Edit /etc/fstab:

Open `/etc/fstab` file with root permissions:

    sudo nano /etc/fstab

And add the following:

    /dev/sdb1    /hdd    ext4    defaults    0    0 

Change the partitions accordingly

### Mount partition:

    sudo mount /hdd


## For mounting existing drive:

    sudo lsblk -f #take a note of the UUID
    sudo nano /etc/fstab
    #add this line in fstab
    UUID=abcd1234-5678-9ef0-1122-334455667788  /mnt/data  ext4  defaults  0  0 #change the UUID
    sudo mount -a

No errors = success ✅