# -*- mode: ruby -*-

Vagrant.configure(2) do |config|
  dirs = ENV['GOPATH'] || Dir.home
  gdir = nil
  config.vm.define "vic_dev" do | vic_dev |
    vic_dev.vm.box = 'boxcutter/ubuntu1504-docker'
    vic_dev.vm.network 'forwarded_port', guest: 2375, host: 12375
    vic_dev.vm.host_name = 'devbox'
    vic_dev.vm.synced_folder '.', '/vagrant', disabled: true
    vic_dev.ssh.username = 'vagrant'

    dirs.split(File::PATH_SEPARATOR).each do |dir|
      gdir = dir.sub("C\:", "/C")
      vic_dev.vm.synced_folder dir, gdir
    end

    vic_dev.vm.provider :virtualbox do |v, _override|
      v.memory = 2048
    end

    [:vmware_fusion, :vmware_workstation].each do |visor|
      vic_dev.vm.provider visor do |v, _override|
        v.memory = 2048
      end
    end

    Dir['machines/devbox/provision*.sh'].each do |path|
      vic_dev.vm.provision 'shell', path: path, args: [gdir, vic_dev.ssh.username]
    end
  end
end
