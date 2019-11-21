#!groovy

stage ('Trigger Build') {
	service = readProperties file: 'service.properties'

	build job: 'Build-Tag-Push-Deploy-QA', wait: true, parameters: [
		[$class: 'StringParameterValue', name: 'PROJECT', value: service.repo]
	]
}
