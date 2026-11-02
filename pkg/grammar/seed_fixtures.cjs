const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
const inputs=['',' ','0','-0','+0','123','-123','+123','00012','-00012','900719925474099312345678901234567890','0xff','0XAB','0o17','0O17','0b101','0B101','+0xff','-0xff','+0o7','-0b1','0x','0o','0b','0xg','0o8','0b2','1n','1.0','1e3','1_000','0x1_0','Infinity','NaN','--1','+','-','1 2','１２','١٢','\u008512\u0085','\u180e12','\u200b12','\ufeff -12 \ufeff']
for(const c of ['\t','\n','\v','\f','\r','\u00a0','\u1680','\u2000','\u2001','\u2002','\u2003','\u2004','\u2005','\u2006','\u2007','\u2008','\u2009','\u200a','\u2028','\u2029','\u202f','\u205f','\u3000','\ufeff'])inputs.push(c+'12'+c)
const cases=inputs.map(input=>{try{return {input,value:BigInt(input).toString()}}catch{return {input,error:'--seed must be a big integer, found: '+input}}})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/seed.json'),JSON.stringify(cases,null,2)+'\n')
