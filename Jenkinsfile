pipeline {
    agent {
        kubernetes {
            label 'go'
        }
    }

    options {
        disableConcurrentBuilds()
    }

    environment {
        http_proxy="http://proxy-dmz.intel.com:912/"
        https_proxy="http://proxy-dmz.intel.com:912/"
        no_proxy="${no_proxy},.intel.com"
        SNYK_API="https://snyk.devtools.intel.com/api"
        SNYK_TOKEN=credentials('SNYK_TOKEN')
        PROTEX_PASSWORD=credentials('PROTEX_CRED')
    }

    stages {

        stage('Prerequisites') {
            steps {
                container('go') {
                    dir("$WORKSPACE") {
                        sh '''
                            curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $(go env GOPATH)/bin
                            curl -fsSL https://deb.nodesource.com/setup_12.x | bash -
                            apt-get install -y yamllint pciutils nodejs aha
                            npm install snyk@v1.658.0 -g
                            npm install snyk-to-html -g
                            mkdir /usr/share/hwdata
                            ln -s /usr/share/misc/pci.ids /usr/share/hwdata/pci.ids
                            update-pciids
                        '''
                    }
                }
            }
        }

        stage ('Build') {
            steps {
                container('go') {
                sh '''
                    go version
                    ci-scripts/sriov-fec_build.sh ${WORKSPACE}
                '''
                }
            }
        }

        stage ('Test') {
            steps {
                container('go') {
                    sh '''
                        ci-scripts/sriov-fec_test.sh ${WORKSPACE}
                    '''
                }
            }
        }

        stage ('go lint') {
            steps {
                container('go') {
                    sh '''
                        ci-scripts/go_lint.sh
                    '''
                }
            }
        }

        stage('Snyk') {
            steps {
                container('go') {
                    sh '''
                        echo "Run Snyk"
                        PROJECT_NAME="sriov-fec-operator"
                        snyk config set endpoint=$SNYK_API
                        snyk --insecure auth $SNYK_TOKEN
                        snyk test --insecure --json --all-projects --detection-depth=10 | snyk-to-html -t /usr/lib/node_modules/snyk-to-html/template/test-cve-report.hbs -o ${WORKSPACE}/${PROJECT_NAME}_snyk_report.html
                        snyk monitor -d --insecure --project="${PROJECT_NAME}" --all-projects --detection-depth=10
                    '''
                }
            }
        }

        stage ('Dockerfile check') {
            steps {
                container('go') {
                    sh '''
                        ci-scripts/dockerfile_check.sh ${WORKSPACE}
                    '''
                }
            }
        }

        stage ('Shell scan') {
            steps {
                container('go') {
                    sh '''
                        ci-scripts/shellcheck-scan.sh ${WORKSPACE}
                    '''
                }
            }
        }

        stage ('Virus / malware scanning') {
            steps {
                container('go') {
                    sh '''
                        ci-scripts/malware_scane.sh ${WORKSPACE}
                    '''
                }
            }
        }

//         stage ('Scan IP') {
//             steps {
//                 container('abi') {
//                     sh '''
//                         cd ${WORKSPACE}
//                         abi ip_scan --context ci-scripts/buildconfig.json --username=sbelhaik --password=$PROTEX_PASSWORD --scan_output="ip_scan"
//                         ci-scripts/check_protex_scan.sh
//                     '''
//                 }
//             }
//         }

        stage('archive') {
          steps {
            archiveArtifacts(artifacts: '**/*.txt, **/*.html, **/bin/, **/*.xlsx', followSymlinks: false)
          }
        }
    }
}

