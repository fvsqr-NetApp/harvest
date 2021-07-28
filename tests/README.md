# NetApp Harvest Tests!

# Git Repo
https://github.com/NetApp/harvest/tests

# Automation Setup

 -  git clone https://github.com/NetApp/harvest/tests
	 - once it is merged into master then use master branch url.
 - install python  >= 3.7 version
-  install pipenv

## TestBed Setup (Assumptions)

By default, these scripts expect the end to end setup is already installed and configured. 
	
 - Download and install harvest

## Configuration 

Once the testbed setup is ready then go to https://github.com/NetApp/harvest/tests/resources/test_params.yml in your workspace then update according to your setup

 - Hosts:
  -
    host_name: harvest-smoke-001
    address:  10.193.113.50
    root_password: netapp
    os_type: RHEL8

## How to run

After setting up both Testbed setup and automation setup, go to root director of the workspace then run 

 - pipenv install
 - pipenv shell
 - set python path "export PYTHONPATH=$PYTHONPATH:<TESTS_DIR>"
 - pytest -v -m smoke
	