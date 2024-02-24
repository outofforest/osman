Name:    osman
Version: %version
Release: 1
Summary: Tool to manage OS images
URL:     https://github.com/outofforest/osman
License: MIT

Requires: zfs libvirt

%description
Tool to manage OS images

%prep
%setup

%setup

%install
mkdir -p %{buildroot}/usr/bin
mkdir -p %{buildroot}/usr/local/lib/systemd/system

cp ./bin/osman-app %{buildroot}/usr/bin/osman
cp ./build/osman.service %{buildroot}/usr/lib/systemd/system/osman.service

%files
/usr/bin/osman
/usr/lib/systemd/system/osman.service

%post
