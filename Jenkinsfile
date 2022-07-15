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
        GO_VERSION="1.18.2"
        GO_TAR="go${GO_VERSION}.linux-amd64.tar.gz"
        GOROOT="/home/jenkins/agent/workspace/go${GO_VERSION}"
        GOPATH="/home/jenkins/agent/workspace/go"
        PATH="${GOROOT}/bin:${env.PATH}"
        SNYK_API="https://snyk.devtools.intel.com/api"
        SNYK_TOKEN=credentials('SNYK_TOKEN')
        PROTEX_PASSWORD=credentials('PROTEX_CRED')
    }

    stages {
        stage ('goinstall') {
            steps {
                container('go') {
                sh '''
                            cd ..
                            mkdir -p ${GOPATH}
                            wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
                            mkdir go${GO_VERSION}
                            tar xzf go${GO_VERSION}.linux-amd64.tar.gz -C /home/jenkins/agent/workspace/go${GO_VERSION} --strip-components=1
                            rm -rf go${GO_VERSION}.linux-amd64.tar.gz
                '''
                }
            }
        }

        stage ('Copyright tag check') {
            steps {
                container('go') {
                    sh '''
                        ci-scripts/copyright_check.sh ${WORKSPACE}
                    '''
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

