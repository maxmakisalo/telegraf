properties(
	[
		buildDiscarder(logRotator(artifactDaysToKeepStr: '', artifactNumToKeepStr: '', daysToKeepStr: '', numToKeepStr: '5')),
		pipelineTriggers
		(
			[
				upstream(threshold: 'SUCCESS', upstreamProjects: 'Marianob85/telegraf/HuaweiHilinkApi, Marianob85/telegraf/OpenHardwareMonitor, Marianob85/telegraf/WebApi '), 
				pollSCM('0 H(5-6) * * *')
			]
		)
	]
)

pipeline
{
	agent { node { label 'linux && stretch' } }
	
	stages
	{
		stage('Checkout')
		{
			steps
			{
				deleteDir()
				dir('src/github.com/influxdata/telegraf') 
				{
					checkout changelog: true, poll: true, scm: 
					[$class: 'GitSCM', branches: 
						[
							[name: '*/master']
						], 
						doGenerateSubmoduleConfigurations: false, extensions: 
						[
							[$class: 'PreBuildMerge', options: [fastForwardMode: 'NO_FF', mergeStrategy: 'RECURSIVE', mergeRemote: 'origin', mergeTarget: 'HuaweiHilinkApi']], 
							[$class: 'PreBuildMerge', options: [fastForwardMode: 'NO_FF', mergeStrategy: 'RECURSIVE', mergeRemote: 'origin', mergeTarget: 'OpenHardwareMonitor']],
							[$class: 'PreBuildMerge', options: [fastForwardMode: 'NO_FF', mergeStrategy: 'RECURSIVE', mergeRemote: 'origin', mergeTarget: 'WebApi']]
						], 
						submoduleCfg: [], userRemoteConfigs: 
						[
							[credentialsId: '5f43e7cc-565c-4d25-adb7-f1f70e87f206', url: 'https://github.com/marianob85/telegraf']
						]
					]
				}
				sh '''
					rm -f -d -r ./release
					rm -f -d -r ./src/github.com/influxdata/telegraf/build
					mkdir ./release'''
			}
		}
		
		stage('Build package') 
		{
			steps
			{
				sh '''
					export GOROOT=/usr/local/go
					export PATH=$PATH:$GOROOT/bin
					export GOPATH=${WORKSPACE}
					workspace=`pwd`
					cd ./src/github.com/influxdata/telegraf
					perl -i -0pe 's/(supported_builds[\\s="\\w:\\[\\],\\{]*linux[:"\\s\\[\\w,]*)/\\1, "mipsle"/im' ./scripts/build.py
					make package
					cd $workspace
					mv ./src/github.com/influxdata/telegraf/build/ ./release/
				'''
			}
		}
		
		stage('Archive')
		{
			steps
			{
				archiveArtifacts artifacts: 'release/**', onlyIfSuccessful: true
			}
		}
		stage('CleanUp')
		{
			steps
			{
				cleanWs()
			}
		}
	}
	post 
	{ 
        failure { 
            notifyFailed()
        }
		success { 
            notifySuccessful()
        }
		unstable { 
            notifyFailed()
        }
    }
}

def notifySuccessful() {
	echo 'Sending e-mail'
	mail (to: 'notifier@manobit.com',
         subject: "Job '${env.JOB_NAME}' (${env.BUILD_NUMBER}) success build",
         body: "Please go to ${env.BUILD_URL}.");
}

def notifyFailed() {
	echo 'Sending e-mail'
	mail (to: 'notifier@manobit.com',
         subject: "Job '${env.JOB_NAME}' (${env.BUILD_NUMBER}) failure",
         body: "Please go to ${env.BUILD_URL}.");
}
