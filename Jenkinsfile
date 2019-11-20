#!groovy

stage ('Trigger Build') {
	build job: 'Build-Tag-Push-Deploy-QA', wait: false, parameters: [
		[$class: 'StringParameterValue', name: 'PROJECT', value: "app-exposer"]
	]
}
