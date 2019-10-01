node {
  stage('Checkout scm') {
    checkout scm
  }

  def img

  stage('Build Docker image') {
    img = docker.build('zyra/gobac')
  }

  stage('Push Docker image') {
    docker.withRegistry('https://harbor.zyra.ca', 'harbor') {
        if (env.BRANCH_NAME == 'master') {
          img.push('latest')
        }
    }
  }
}