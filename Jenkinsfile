def updatePRStatus(String status){

    switch(status) {
        case 'running':
            script{
                gitHubPRStatus githubPRMessage("[Jenkins] job is running...")
            }
            break
        case 'success':
            script{
                githubPRStatusPublisher(
                    statusMsg: [content: "[Jenkins] build ended with SUCCESS"], 
                    unstableAs: 'FAILURE',
                    buildMessage: 
                        [
                            failureMsg: [content: 'Can\'t set status; build failed!.'], 
                            successMsg: [content: 'Successfully set status; build succeeded.']
                        ],
                )     
            }
            break;
        case 'failure':
            script{
                githubPRStatusPublisher(
                    statusMsg: [content: "[Jenkins] build ended with FAILURE"], 
                    unstableAs: 'FAILURE',
                    buildMessage: 
                        [
                            failureMsg: [content: 'Can\'t set status; build failed!.'], 
                            successMsg: [content: 'Successfully set status; build succeeded.']
                        ]
                )     
            }
            break;
        case 'aborted':
            script{
                githubPRStatusPublisher(
                    statusMsg: [content: "[Jenkins] build was ABORTED"], 
                    unstableAs: 'FAILURE',
                    buildMessage: 
                        [
                            failureMsg: [content: 'Can\'t set status; build failed!.'], 
                            successMsg: [content: 'Successfully set status; build succeeded.']
                        ]
                )     
            }
            break;
        case 'unstable':
            script{
                githubPRStatusPublisher(
                    statusMsg: [content: "[Jenkins] build is UNSTABLE"], 
                    unstableAs: 'FAILURE',
                    buildMessage: 
                        [
                            failureMsg: [content: 'Can\'t set status; build failed!.'], 
                            successMsg: [content: 'Successfully set status; build succeeded.']
                        ]
                )     
            }
            break;
        default:
            break;
    }

}

currentBuild.displayName = "${env.BUILD_NUMBER}-${env.GITHUB_PR_SOURCE_BRANCH ? env.GITHUB_PR_SOURCE_BRANCH : params.BRANCH ? params.BRANCH : 'main'}"

def isManual = currentBuild.rawBuild.getCauses()[0].toString().contains('UserIdCause')

pipeline {
    agent {
        kubernetes {
            label 'go'
        }
    }

    options {
        disableConcurrentBuilds()
        timestamps()
        skipDefaultCheckout()
        timeout(time: 1, unit: 'HOURS')
    }

    parameters {
        string(name: 'REPO_URL', defaultValue: "https://github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator.git", description: 'Please provide the repository URL')
        string(name: 'REPO_NAME', defaultValue: "sriov-fec-operator", description: 'Please provide the repository name')
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
        GOLANGCI_LINT_VERSION="1.47.3"
        SNYK_API="https://snyk.devtools.intel.com/api"
        SNYK_TOKEN=credentials('SNYK_TOKEN')
        PROTEX_PASSWORD=credentials('PROTEX_CRED')
        REPO_DIR="sriov-fec-operator"
        BDBA_TOKEN=credentials('BDBA_TOKEN')
        BDBA_URL="https://bdba001.icloud.intel.com"
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

        stage('Check-out'){
            steps{
                checkout([
                    $class: 'GitSCM', 
                    branches: [[name: "${env.GITHUB_PR_SOURCE_BRANCH ? env.GITHUB_PR_SOURCE_BRANCH : params.BRANCH ? params.BRANCH : 'main'}"]], 
                    doGenerateSubmoduleConfigurations: false, 
                    extensions: [[
                        $class: 'RelativeTargetDirectory', 
                        relativeTargetDir: params.REPO_NAME]], 
                    gitTool: 'Default', 
                    submoduleCfg: [], 
                    userRemoteConfigs: [[
                        credentialsId: 'gh_jenkins_auth',
                        url: params.REPO_URL]]])
            }
        }

        stage('Update PR status'){
            steps{
                script{
                    if ( isManual ) {
                        println "Manual run, skipping..."
                    } else {
                        updatePRStatus('running')
                    }
                }
            }
        }

        stage ('Copyright tag check') {
            steps {
                container('go') {
                    sh '''
                        cd ${REPO_DIR}
                        ci-scripts/copyright_check.sh .
                    '''
                }
            }
        }

        stage ('Build') {
            steps {
                container('go') {
                sh '''
                    go version
                    cd ${REPO_DIR}
                    ci-scripts/sriov-fec_build.sh .
                '''
                }
            }
        }

        stage ('Test') {
            steps {
                container('go') {
                    sh '''
                        cd ${REPO_DIR}
                        ci-scripts/sriov-fec_test.sh .
                    '''
                }
            }
        }

        stage('pf-bb-config BDBA Scan') {
            steps {
                container('abi') {
                    sh '''
                        cd .. && git clone https://github.com/intel/pf-bb-config
                        cd pf-bb-config
                        ./build.sh
                        zip pf_bb_config.zip pf_bb_config
                        ls -l
                        abi binary_scan scan --zip_file pf_bb_config.zip --report_name "pf_bb_config_bdba_report" --include_html --include_components --api_token $BDBA_TOKEN --tool_url $BDBA_URL --tool_group 32 --timeout 20
                     '''
                }
            }
        }

        stage('BDBA Scan') {
            steps {
                container('abi') {
                    sh '''
                        cd ${REPO_DIR}
                        rm -r .git Makefile spec/sriov-fec-selector-based-api.puml spec/images/*
                        find . -name main.go -type f -delete
                        zip -r -q sriov-fec.zip . 
                        ls -la
                        abi binary_scan scan --zip_file sriov-fec.zip --report_name "sriov-fec_bdba_report" --include_html --include_components --api_token $BDBA_TOKEN --tool_url $BDBA_URL --tool_group 32 --timeout 20
                     '''
                }
            }
        }

        stage ('go lint') {
            steps {
                container('go') {
                    sh '''
                        cd ${REPO_DIR}
                        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v${GOLANGCI_LINT_VERSION}
                        export PATH=$PATH:$(go env GOPATH)/bin
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
                        snyk test --insecure --json --all-projects --detection-depth=10 | snyk-to-html -t /usr/lib/node_modules/snyk-to-html/template/test-cve-report.hbs -o ${REPO_DIR}/${PROJECT_NAME}_snyk_report.html
                        snyk monitor -d --insecure --project="${PROJECT_NAME}" --all-projects --detection-depth=10
                        
                        cd ${REPO_DIR}
                        ci-scripts/check_snyk_scan.sh ${PROJECT_NAME}_snyk_report.html
                    '''
                }
            }
        }

        stage ('Dockerfile check') {
            steps {
                container('go') {
                    sh '''
                        cd ${REPO_DIR}
                        ci-scripts/dockerfile_check.sh .
                    '''
                }
            }
        }

        stage ('Shell scan') {
            steps {
                container('go') {
                    sh '''
                        cd ${REPO_DIR}
                        ci-scripts/shellcheck-scan.sh .
                    '''
                }
            }
        }

        stage('Kubesec Scan') {
            steps {
                container('go') {
                    sh '''
                        cd ${REPO_DIR}
                        wget -O kubesec_linux_amd64.tar.gz https://github.com/controlplaneio/kubesec/releases/download/v2.11.4/kubesec_linux_amd64.tar.gz
                        tar xvzf kubesec_linux_amd64.tar.gz
                        cp kubesec /usr/local/bin/
                        find . -type f \\( -iname "*.yaml" -o -iname "*.yml" \\) -print -exec kubesec scan {} ';' | tee kubesec.log
                     '''
                }
            }
        }

        stage ('Virus / malware scanning') {
            steps {
                container('go') {
                    sh '''
                        cd ${REPO_DIR}
                        ci-scripts/malware_scane.sh .
                    '''
                }
            }
        }

         stage ('Protex') {
             steps {
                 catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                     container('abi') {
                         sh '''
                             cd ${REPO_DIR}
                             abi ip_scan --context ci-scripts/buildconfig.json --username=sbelhaik --password=$PROTEX_PASSWORD --scan_output="ip_scan"
                             ci-scripts/check_protex_scan.sh
                         '''
                     }
                 }
             }
         }

        stage('archive') {
          steps {
            archiveArtifacts(artifacts: '**/*.txt, **/*.html, **/bin/, **/*.xlsx, **/*.log', followSymlinks: false)
          }
        }
    }

    post{
        success {
            script{
                if ( isManual ) {
                    println "Manual run, skipping..."
                } else { updatePRStatus('success') }
            }
        }
        failure {
            script{
                if ( isManual ) {
                    println "Manual run, skipping..."
                } else { updatePRStatus('failure') }
            }
        }
        aborted {
            script{
                if ( isManual ) {
                    println "Manual run, skipping..."
                } else { updatePRStatus('aborted') }
            }
        }
        unstable {
            script{
                if ( isManual ) {
                    println "Manual run, skipping..."
                } else { updatePRStatus('unstable') }
            }
        }
    }

}

