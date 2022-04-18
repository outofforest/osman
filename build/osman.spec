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

%files
/usr/bin/osman

%post
