import {readFileSync} from 'node:fs'

const file=process.argv[2]
if(!file)throw new Error('package-lock path required')
const lock=JSON.parse(readFileSync(file,'utf8'))
const packages=['@wailsio/runtime','react','react-dom','scheduler'].map(name=>({
  name,
  version:lock.packages[`node_modules/${name}`]?.version,
}))
if(packages.some(pkg=>!pkg.version))throw new Error('Frontend runtime dependency missing from lockfile')
process.stdout.write(JSON.stringify(packages))
