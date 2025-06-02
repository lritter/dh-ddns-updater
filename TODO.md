# Things that would be nice to have in the future

- I'm not sure the installation process always creates files with the right
  permissions. Maybe we should add a `postinst` script to set permissions?
- Add a `preinst` script to check for `jq` and `curl` dependencies.
- Add a smart upgrade mechanism and add the ability to build in a version to the 
  binary as well as well as a version flag.
  