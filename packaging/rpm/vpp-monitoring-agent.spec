%define _version %(./version)
%define _release %(./release)

Name:       vpp-monitoring-agent
Version:    %{_version}
# The Fedora/CentOS packaging guidelines *require* the use of a disttag. Vpp-monitoring-agent's
#   RPM build doesn't do anything Fedora/CentOS specific, so the disttag is
#   unnecessary and unused in our case, but both the docs and the pros (apevec)
#   agree that we should include it.
# See: https://fedoraproject.org/wiki/Packaging:DistTag
Release:    %{_release}
Summary:    pnda VPP-monitoring-agent
Group:      Applications/Communications
License:    Copyright PNDA.io pnda
URL:        http://www.pnda
Source0:    vpp-monitoring-agent
Source1:    vpp-monitoring-agent.service
Source2:    vpp-monitoring-agent-configuration.yaml
Source3:    vpp-monitoring-agent.sh
Requires:   vpp >= 17.01, vpp < 17.04
# Required for creating vpp-monitoring-agent group
Requires(pre): shadow-utils
# Required for configuring systemd
BuildRequires: systemd

%pre
# Create `vpp-monitoring-agent` user/group
# Short circuits if the user/group already exists
# Home dir must be a valid path for various files to be created in it
getent passwd vpp-monitoring-agent > /dev/null || useradd vpp-monitoring-agent -M -d $RPM_BUILD_ROOT/opt/%name
getent group vpp-monitoring-agent > /dev/null || groupadd vpp-monitoring-agent
getent group vpp > /dev/null && usermod -a -G vpp vpp-monitoring-agent

%description
pnda VPP-monitoring-agent

%install
# Create directory in build root for Vpp-monitoring-agent
mkdir -p $RPM_BUILD_ROOT/opt/%name
# Copy Vpp-monitoring-agent from archive to its dir in build root
cp -r ../SOURCES/* $RPM_BUILD_ROOT/opt/%name
# Create directory in build root for systemd .service file
mkdir -p $RPM_BUILD_ROOT/%{_unitdir}
# Copy Vpp-monitoring-agent's systemd .service file to correct dir in build root
echo "PWD:$PWD"
cp ${RPM_BUILD_ROOT}/../../%{name}.service $RPM_BUILD_ROOT/%{_unitdir}/%name.service

%postun
#   When the RPM is removed, the subdirs containing new files wouldn't normally
#   be deleted. Manually clean them up.
#   Warning: This does assume there's no data there that should be preserved
if [ $1 -eq 0 ]; then
    rm -rf $RPM_BUILD_ROOT/opt/%name
fi

%files
# Vpp-monitoring-agent will run as vpp-monitoring-agent:vpp-monitoring-agent, set as user:group for vpp-monitoring-agent dir, don't override mode
%attr(-,vpp-monitoring-agent,vpp-monitoring-agent) /opt/%name
# Configure systemd unitfile user/group/mode
%attr(0644,root,root) %{_unitdir}/%name.service

