node {
  checkout scm

  def img = docker.build('gobac')

  docker.withRegistry('https://registry.tor1.zyra.ca:5000') {
    if (env.BRANCH_NAME == 'master') {
      img.push('latest')
    }
  }

}