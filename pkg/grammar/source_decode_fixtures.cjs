// Node is the upstream file-source decoder; retain exact malformed-byte behavior.
const fs=require('fs')
const path=require('path')
const cases=['','61','efbbbf61','e282','e228a1','f09080','eda080','f4908080','c080','8080','e18041','f1808041','e08080','f0808080','f48fbfbf','f0908080','eda0','efbfbd','c2a2','e282ac','f09f9880']
// All possible leading bytes, with EOF, ASCII interruption, and continuations.
for(let byte=0;byte<256;byte++)for(const suffix of ['','41','808080','a08080','bf8080'])cases.push(byte.toString(16).padStart(2,'0')+suffix)
const fixtures=[...new Set(cases)].map(hex=>({hex,text:Buffer.from(hex,'hex').toString('utf8')}))
fs.writeFileSync(path.join(__dirname,'../testdata/parser/utf8.json'),JSON.stringify(fixtures)+'\n')
