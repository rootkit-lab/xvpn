#!/usr/bin/env node
"use strict";

// OpenVSCode 1.98 só declara viewsContainers.activitybar (esquerda) e
// .panel (embaixo). additionalProperties:false descarta secondarySideBar,
// e a view acaba no Explorer. Este patch registra a auxiliary bar (direita).

const fs = require("fs");

const TARGET =
  process.argv[2] ||
  "/home/.openvscode-server/out/vs/code/browser/workbench/workbench.js";

const SCHEMA_FROM =
  'pfs={description:d(2715,null),type:"object",properties:{activitybar:{description:d(2716,null),type:"array",items:kGt},panel:{description:d(2717,null),type:"array",items:kGt}},additionalProperties:!1}';
const SCHEMA_TO =
  'pfs={description:d(2715,null),type:"object",properties:{activitybar:{description:d(2716,null),type:"array",items:kGt},panel:{description:d(2717,null),type:"array",items:kGt},secondarySideBar:{description:d(2717,null),type:"array",items:kGt},secondarySidebar:{description:d(2717,null),type:"array",items:kGt}},additionalProperties:!1}';

const SWITCH_FROM =
  'case"activitybar":n=this.l(h,l,n,t,0);break;case"panel":r=this.l(h,l,r,t,1);break}';
const SWITCH_TO =
  'case"activitybar":n=this.l(h,l,n,t,0);break;case"panel":r=this.l(h,l,r,t,1);break;case"secondarySideBar":case"secondarySidebar":this.l(h,l,0,t,2);break}';

const src = fs.readFileSync(TARGET, "utf8");
if (src.includes('case"secondarySideBar"') && src.includes("secondarySideBar:{description")) {
  console.log("patch-auxiliary-bar: already applied");
  process.exit(0);
}
if (!src.includes(SCHEMA_FROM) || !src.includes(SWITCH_FROM)) {
  console.error("patch-auxiliary-bar: OpenVSCode workbench.js changed; update needles");
  process.exit(1);
}

const out = src.replace(SCHEMA_FROM, SCHEMA_TO).replace(SWITCH_FROM, SWITCH_TO);
fs.writeFileSync(TARGET, out);
console.log("patch-auxiliary-bar: registered secondarySideBar → AuxiliaryBar");
