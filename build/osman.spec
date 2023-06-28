Name:    osman
Version: %version
Release: 1
Summary: Tool to manage OS images
URL:     https://github.com/outofforest/osman
License: MIT

Requires: zfs

%description
Tool to manage OS images

%prep
%setup

%setup

%install
mkdir -p %{buildroot}/usr/bin
cp ./bin/osman-app %{buildroot}/usr/bin/osman
cp ./build/osman-autostart.service %{buildroot}/usr/local/lib/systemd/system/osman-autostart.service

%files
/usr/bin/osman
/usr/local/lib/systemd/system/osman-autostart.service

%post
