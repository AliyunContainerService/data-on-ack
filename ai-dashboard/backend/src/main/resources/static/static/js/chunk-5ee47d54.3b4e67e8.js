(window["webpackJsonp"]=window["webpackJsonp"]||[]).push([["chunk-5ee47d54"],{"0481":function(e,t,r){"use strict";var n=r("23e7"),a=r("a2bf"),i=r("7b0b"),o=r("50c4"),u=r("a691"),s=r("65f0");n({target:"Array",proto:!0},{flat:function(){var e=arguments.length?arguments[0]:void 0,t=i(this),r=o(t.length),n=s(t,0);return n.length=a(n,t,t,r,0,void 0===e?1:u(e)),n}})},1276:function(e,t,r){"use strict";var n=r("d784"),a=r("44e7"),i=r("825a"),o=r("1d80"),u=r("4840"),s=r("8aa5"),l=r("50c4"),c=r("14c3"),d=r("9263"),f=r("d039"),m=[].push,p=Math.min,h=4294967295,g=!f((function(){return!RegExp(h,"y")}));n("split",2,(function(e,t,r){var n;return n="c"=="abbc".split(/(b)*/)[1]||4!="test".split(/(?:)/,-1).length||2!="ab".split(/(?:ab)*/).length||4!=".".split(/(.?)(.?)/).length||".".split(/()()/).length>1||"".split(/.?/).length?function(e,r){var n=String(o(this)),i=void 0===r?h:r>>>0;if(0===i)return[];if(void 0===e)return[n];if(!a(e))return t.call(n,e,i);var u,s,l,c=[],f=(e.ignoreCase?"i":"")+(e.multiline?"m":"")+(e.unicode?"u":"")+(e.sticky?"y":""),p=0,g=new RegExp(e.source,f+"g");while(u=d.call(g,n)){if(s=g.lastIndex,s>p&&(c.push(n.slice(p,u.index)),u.length>1&&u.index<n.length&&m.apply(c,u.slice(1)),l=u[0].length,p=s,c.length>=i))break;g.lastIndex===u.index&&g.lastIndex++}return p===n.length?!l&&g.test("")||c.push(""):c.push(n.slice(p)),c.length>i?c.slice(0,i):c}:"0".split(void 0,0).length?function(e,r){return void 0===e&&0===r?[]:t.call(this,e,r)}:t,[function(t,r){var a=o(this),i=void 0==t?void 0:t[e];return void 0!==i?i.call(t,a,r):n.call(String(a),t,r)},function(e,a){var o=r(n,e,this,a,n!==t);if(o.done)return o.value;var d=i(e),f=String(this),m=u(d,RegExp),b=d.unicode,v=(d.ignoreCase?"i":"")+(d.multiline?"m":"")+(d.unicode?"u":"")+(g?"y":"g"),y=new m(g?d:"^(?:"+d.source+")",v),$=void 0===a?h:a>>>0;if(0===$)return[];if(0===f.length)return null===c(y,f)?[f]:[];var N=0,O=0,j=[];while(O<f.length){y.lastIndex=g?O:0;var x,S=c(y,g?f:f.slice(O));if(null===S||(x=p(l(y.lastIndex+(g?0:O)),f.length))===N)O=s(f,O,b);else{if(j.push(f.slice(N,O)),j.length===$)return j;for(var w=1;w<=S.length-1;w++)if(j.push(S[w]),j.length===$)return j;O=N=x}}return j.push(f.slice(N)),j}]}),!g)},"23ba":function(e,t,r){"use strict";r.d(t,"h",(function(){return c})),r.d(t,"g",(function(){return m})),r.d(t,"a",(function(){return p})),r.d(t,"e",(function(){return h})),r.d(t,"b",(function(){return g})),r.d(t,"f",(function(){return b})),r.d(t,"c",(function(){return y})),r.d(t,"d",(function(){return $})),r.d(t,"i",(function(){return N}));var n=r("b85c"),a=r("2909"),i=r("3835"),o=(r("4fad"),r("ac1f"),r("466d"),r("1276"),r("a15b"),r("fb6a"),r("b0c0"),r("e9c4"),r("99af"),r("a434"),r("d3b7"),r("159b"),r("c740"),r("caad"),r("2532"),r("ed08")),u=r("61f7"),s=r("b775");function l(e,t){if(void 0!==e)for(var r=0,n=Object.entries(e);r<n.length;r++){var a=n[r],i=a[0];t.root.indexOf(i)<0&&t.root.push(i)}}function c(e){if(Object(o["b"])(e)||Object(o["b"])(e.memory))return e;var t=e.memory,r=/(^(\d{1,})\.?(\d{0,})$)|(^N\/A$)/g,n=t.match(r);return null!==n&&(e.memory=t+"M"),e}function d(e){var t=arguments.length>1&&void 0!==arguments[1]?arguments[1]:".",r=e.split(t),n="";return r.length>1&&(n=r.slice(0,r.length-1).join(t)),[n,r[r.length-1]]}function f(e,t){return Object(o["b"])(e)?t:e+"."+t}function m(e,t){if(Object(o["b"])(e)||Object(o["b"])(e.name))return t;var r=JSON.parse(JSON.stringify(e));if(r.name=f(e.prefix,e.name),r.min=c(r.min),r.max=c(r.max),t=r,Object(o["b"])(e.children))return t;t.children=[];for(var n=0;n<e.children.length;n++){var a=e.children[n];t.children.push(m(a,null))}return t}function p(e){var t=e.k8sVersion;return Object(u["b"])(t,"1.20.0")>=0}function h(e,t,r){void 0===t&&(t={root:[]}),void 0===r&&(r={root:[]});var n={};if(void 0===e.name)return[n,t,r];var u=d(e.name),s=Object(i["a"])(u,2),c=s[0],f=s[1];if(n.name=f,n.prefix=c,n.min=e.min,l(n.min,t),n.max=e.max,l(n.max,t),void 0!==e.namespaces&&(n.namespaces=e.namespaces,r.root=[].concat(Object(a["a"])(r.root),Object(a["a"])(n.namespaces))),!Object(o["b"])(e.children)){void 0===n.children&&(n.children=[]);for(var m=0;m<e.children.length;m++){var p,g=e.children[m],b=h(g,t,r),v=Object(i["a"])(b,3);p=v[0],t=v[1],r=v[2],n.children.push(p)}}return[n,t,r]}function g(e,t){var r=arguments.length>2&&void 0!==arguments[2]?arguments[2]:"",n=e.children||[],a=r.split(".");a.splice(0,1);var i=0;if(a.length>0)while(i<a.length)n.forEach((function(e){e.name===a[i]&&(n=e.children||[],i++)}));var o=n.findIndex((function(e){return e.name===t}));return o<0?(console.log("not found node name:",t,r),e):(n.splice(o,1),e)}function b(){var e,t=arguments.length>0&&void 0!==arguments[0]?arguments[0]:"prefix",r=arguments.length>1?arguments[1]:void 0,n=arguments.length>2?arguments[2]:void 0,a=function r(a){a.name===n||(a.name||"").includes(n)?e="prefix"===t?a.prefix:a:a.children&&a.children.forEach((function(e){return r(e)}))};return a(r),e}var v=function e(t){var r=arguments.length>1&&void 0!==arguments[1]?arguments[1]:[];if((!t.children||t.children&&0===t.children.length)&&r.push(f(t.prefix,t.name)),t.children){var a,i=Object(n["a"])(t.children);try{for(i.s();!(a=i.n()).done;){var o=a.value;e(o,r)}}catch(u){i.e(u)}finally{i.f()}}};function y(){var e=arguments.length>0&&void 0!==arguments[0]?arguments[0]:{},t=[];return e.spec&&e.spec.root&&v(e.spec.root,t),t}function $(e){return Object(s["a"])({url:"/group/list",method:"get",params:e})}function N(e,t,r,n,a){return Object(o["b"])(n)&&(n=r),Object(s["a"])({url:"/group/update",method:"put",params:{oldNodeName:r,newNodeName:n,prefix:a,action:t},data:e})}},2909:function(e,t,r){"use strict";r.d(t,"a",(function(){return s}));var n=r("6b75");function a(e){if(Array.isArray(e))return Object(n["a"])(e)}r("a4d3"),r("e01a"),r("d3b7"),r("d28b"),r("3ca3"),r("ddb0"),r("a630");function i(e){if("undefined"!==typeof Symbol&&null!=e[Symbol.iterator]||null!=e["@@iterator"])return Array.from(e)}var o=r("06c5");function u(){throw new TypeError("Invalid attempt to spread non-iterable instance.\nIn order to be iterable, non-array objects must have a [Symbol.iterator]() method.")}function s(e){return a(e)||i(e)||Object(o["a"])(e)||u()}},"333d":function(e,t,r){"use strict";var n=function(){var e=this,t=e.$createElement,r=e._self._c||t;return r("div",{staticClass:"pagination-container",class:{hidden:e.hidden}},[r("el-pagination",e._b({attrs:{background:e.background,"current-page":e.currentPage,"page-size":e.pageSize,layout:e.layout,"page-sizes":e.pageSizes,total:e.total},on:{"update:currentPage":function(t){e.currentPage=t},"update:current-page":function(t){e.currentPage=t},"update:pageSize":function(t){e.pageSize=t},"update:page-size":function(t){e.pageSize=t},"size-change":e.handleSizeChange,"current-change":e.handleCurrentChange}},"el-pagination",e.$attrs,!1))],1)},a=[];r("a9e3");Math.easeInOutQuad=function(e,t,r,n){return e/=n/2,e<1?r/2*e*e+t:(e--,-r/2*(e*(e-2)-1)+t)};var i=function(){return window.requestAnimationFrame||window.webkitRequestAnimationFrame||window.mozRequestAnimationFrame||function(e){window.setTimeout(e,1e3/60)}}();function o(e){document.documentElement.scrollTop=e,document.body.parentNode.scrollTop=e,document.body.scrollTop=e}function u(){return document.documentElement.scrollTop||document.body.parentNode.scrollTop||document.body.scrollTop}function s(e,t,r){var n=u(),a=e-n,s=20,l=0;t="undefined"===typeof t?500:t;var c=function e(){l+=s;var u=Math.easeInOutQuad(l,n,a,t);o(u),l<t?i(e):r&&"function"===typeof r&&r()};c()}var l={name:"Pagination",props:{total:{required:!0,type:Number},page:{type:Number,default:1},limit:{type:Number,default:20},pageSizes:{type:Array,default:function(){return[10,20,30,50]}},layout:{type:String,default:"total, sizes, prev, pager, next, jumper"},background:{type:Boolean,default:!0},autoScroll:{type:Boolean,default:!0},hidden:{type:Boolean,default:!1}},computed:{currentPage:{get:function(){return this.page},set:function(e){this.$emit("update:page",e)}},pageSize:{get:function(){return this.limit},set:function(e){this.$emit("update:limit",e)}}},methods:{handleSizeChange:function(e){this.$emit("pagination",{page:this.currentPage,limit:e}),this.autoScroll&&s(0,800)},handleCurrentChange:function(e){this.$emit("pagination",{page:e,limit:this.pageSize}),this.autoScroll&&s(0,800)}}},c=l,d=(r("5660"),r("2877")),f=Object(d["a"])(c,n,a,!1,null,"6af373ef",null);t["a"]=f.exports},3872:function(e,t,r){},4069:function(e,t,r){var n=r("44d2");n("flat")},"466d":function(e,t,r){"use strict";var n=r("d784"),a=r("825a"),i=r("50c4"),o=r("1d80"),u=r("8aa5"),s=r("14c3");n("match",1,(function(e,t,r){return[function(t){var r=o(this),n=void 0==t?void 0:t[e];return void 0!==n?n.call(t,r):new RegExp(t)[e](String(r))},function(e){var n=r(t,e,this);if(n.done)return n.value;var o=a(e),l=String(this);if(!o.global)return s(o,l);var c=o.unicode;o.lastIndex=0;var d,f=[],m=0;while(null!==(d=s(o,l))){var p=String(d[0]);f[m]=p,""===p&&(o.lastIndex=u(l,i(o.lastIndex),c)),m++}return 0===m?null:f}]}))},4820:function(e,t,r){"use strict";r.d(t,"e",(function(){return i})),r.d(t,"c",(function(){return u})),r.d(t,"a",(function(){return s})),r.d(t,"f",(function(){return l})),r.d(t,"b",(function(){return c})),r.d(t,"d",(function(){return d}));r("b0c0");var n=r("b775"),a=r("ed08");function i(e,t){var r=e.spec;if(Object(a["b"])(r))return{name:"",userNames:[],quotaNames:""};var n=r.groupName;return{metaName:e.metadata.name,metaNamepace:e.metadata.namespace,name:n,userNames:t,quotaNames:Object(a["b"])(r.quotaNames)?"":r.quotaNames[0]}}function o(e){var t=Object(a["b"])(e.metaName)?e.name:e.metaName,r=Object(a["b"])(e.metaNamespace)?"kube-ai":e.metaNamespace,n={kind:"UserGroup",apiVersion:"data.kubeai.alibabacloud.com/v1",metadata:{name:t,namespace:r},spec:{groupName:e.name}};return e.quotaNames&&(n.spec.quotaNames=e.quotaNames.constructor===Array?e.quotaNames:[e.quotaNames]),n}function u(e){return Object(n["a"])({url:"/user_group/list",method:"get",params:e})}function s(e,t){var r=o(e);return Object(n["a"])({url:"/user_group/create",method:"post",data:{userGroup:r,userNames:t}})}function l(e,t){var r={userGroup:o(e),users:t};return Object(n["a"])({url:"/user_group/update",method:"put",data:r})}function c(e){var t=o(e);return Object(n["a"])({url:"/user_group/delete",method:"put",data:t})}function d(e){return Object(n["a"])({url:"/user_group/get_group_namespaces",method:"get",params:e})}},"4ec9":function(e,t,r){"use strict";var n=r("6d61"),a=r("6566");e.exports=n("Map",(function(e){return function(){return e(this,arguments.length?arguments[0]:void 0)}}),a)},"4fad":function(e,t,r){var n=r("23e7"),a=r("6f53").entries;n({target:"Object",stat:!0},{entries:function(e){return a(e)}})},5660:function(e,t,r){"use strict";r("7a30")},"6e6a":function(e,t,r){"use strict";r.r(t),r.d(t,"constFormTemplate",(function(){return m}));var n=function(){var e=this,t=e.$createElement,r=e._self._c||t;return r("div",{staticClass:"app-container"},[r("div",{staticClass:"filter-container"},[r("el-input",{staticClass:"filter-item",staticStyle:{width:"200px"},attrs:{placeholder:e.$t("userGroup.name")},nativeOn:{keyup:function(t){return!t.type.indexOf("key")&&e._k(t.keyCode,"enter",13,t.key,"Enter")?null:e.fetchData(t)}},model:{value:e.listQuery.userGroupName,callback:function(t){e.$set(e.listQuery,"userGroupName",t)},expression:"listQuery.userGroupName"}}),r("el-button",{staticClass:"filter-item",staticStyle:{"margin-left":"10px"},attrs:{type:"primary",icon:"el-icon-search"},on:{click:e.fetchData}},[e._v(" "+e._s(e.$t("user.search"))+" ")]),r("el-button",{staticClass:"filter-item",staticStyle:{"margin-left":"10px"},attrs:{type:"primary",icon:"el-icon-edit"},on:{click:e.handleCreate}},[e._v(" "+e._s(e.$t("user.add"))+" ")]),r("el-button",{staticClass:"fr",staticStyle:{"margin-left":"10px"},attrs:{type:"default",icon:"el-icon-refresh-left"},on:{click:e.refresh}},[e._v(" "+e._s(e.$t("userGroup.refresh"))+" ")])],1),r("el-table",{directives:[{name:"loading",rawName:"v-loading",value:e.listLoading,expression:"listLoading"}],staticStyle:{"margin-top":"20px"},attrs:{data:e.list,"element-loading-text":"Loading",border:"",fit:"","highlight-current-row":"","row-key":"id"}},[r("el-table-column",{attrs:{prop:"spec.groupName",label:e.$t("userGroup.name")}}),r("el-table-column",{attrs:{label:e.$t("user.quota")},scopedSlots:e._u([{key:"default",fn:function(t){return e._l(t.row.spec.quotaNames,(function(t){return r("span",{key:t,staticClass:"mr2"},[e._v(" "+e._s(t)+" ")])}))}}])}),r("el-table-column",{attrs:{label:e.$t("userGroup.user")},scopedSlots:e._u([{key:"default",fn:function(t){return e._l(e.getUsersByGroup(t.row.metadata.name,e.userList),(function(t){return r("div",{key:t,staticClass:"mr2"},[e._v(" "+e._s(t)+" ")])}))}}])}),r("el-table-column",{attrs:{prop:"metadata.creationTimestamp",label:e.$t("user.createTime"),width:"180"}}),r("el-table-column",{attrs:{label:e.$t("user.operator"),align:"center",width:"160","class-name":"small-padding fixed-width"},scopedSlots:e._u([{key:"default",fn:function(t){return[r("el-button",{attrs:{type:"primary",size:"mini"},on:{click:function(r){return e.handleUpdate(t.row)}}},[e._v(" "+e._s(e.$t("user.edit"))+" ")]),r("el-button",{attrs:{size:"mini",type:"danger"},on:{click:function(r){return e.handleDelete(t.row,t.$index)}}},[e._v(" "+e._s(e.$t("user.delete"))+" ")])]}}])})],1),r("pagination",{directives:[{name:"show",rawName:"v-show",value:e.list.length>0,expression:"list.length > 0"}],attrs:{total:e.list.length,page:e.listQuery.page,limit:e.listQuery.limit},on:{"update:page":function(t){return e.$set(e.listQuery,"page",t)},"update:limit":function(t){return e.$set(e.listQuery,"limit",t)},pagination:e.fetchData}}),r("el-dialog",{attrs:{title:e.textMap[e.dialogStatus],visible:e.dialogFormVisible},on:{"update:visible":function(t){e.dialogFormVisible=t}}},[r("el-form",{ref:"dataForm",staticStyle:{"margin-left":"20px"},attrs:{model:e.formTemplate,"label-position":"left","label-width":"120px"}},[r("el-form-item",{attrs:{label:e.$t("userGroup.name"),prop:"name",rules:[{required:!0,message:e.$t("userGroup.nameEmptyNotice"),trigger:"blur"},{min:1,max:63,message:e.$t("userGroup.invalidNameLength"),trigger:"blur"},{message:e.$t("userGroup.invalidNamePattern"),trigger:"blur",type:"string",pattern:/^[0-9a-zA-Z][0-9a-zA-Z-]*$/}]}},[r("el-input",{attrs:{placeholder:e.$t("userGroup.namePlaceholder")},model:{value:e.formTemplate.name,callback:function(t){e.$set(e.formTemplate,"name",t)},expression:"formTemplate.name"}})],1),r("el-form-item",{attrs:{label:e.$t("userGroup.quotaNode"),prop:"quotaNames",rules:{required:!0,validator:e.quotaNamesValidator,message:e.$t("userGroup.quotaEmptyNotice"),trigger:"blur"}}},["update"===e.dialogStatus?r("el-input",{attrs:{placeholder:e.formTemplate.quotaNames,disabled:!0}}):r("el-select",{attrs:{placeholder:e.$t("userGroup.quotaNames")},model:{value:e.formTemplate.quotaNames,callback:function(t){e.$set(e.formTemplate,"quotaNames",t)},expression:"formTemplate.quotaNames"}},e._l(e.filterQuotaNodeByGroup(e.quotaList),(function(e){return r("el-option",{key:e,attrs:{label:e,value:e}})})),1)],1),r("el-form-item",{attrs:{label:e.$t("userGroup.user")}},[r("el-select",{attrs:{placeholder:e.$t("userGroup.userNames"),multiple:""},model:{value:e.formTemplate.userNames,callback:function(t){e.$set(e.formTemplate,"userNames",t)},expression:"formTemplate.userNames"}},e._l(e.userList,(function(e){return r("el-option",{key:e.spec.userId,attrs:{label:e.spec.userName,value:e.spec.userName}})})),1)],1)],1),r("div",{staticClass:"dialog-footer",attrs:{slot:"footer"},slot:"footer"},[r("el-button",{on:{click:function(t){e.dialogFormVisible=!1}}},[e._v(" "+e._s(e.$t("user.cancel"))+" ")]),r("el-button",{attrs:{type:"primary"},on:{click:function(t){"create"===e.dialogStatus?e.createData():e.updateData()}}},[e._v(" "+e._s(e.$t("user.save"))+" ")])],1)],1)],1)},a=[],i=r("2909"),o=(r("e9c4"),r("d3b7"),r("0481"),r("4069"),r("d81d"),r("4de4"),r("caad"),r("2532"),r("b0c0"),r("ed08")),u=r("bc3a"),s=r.n(u),l=r("333d"),c=r("d3a2"),d=r("4820"),f=r("23ba"),m={name:"",userNames:[],quotaNames:""},p={page:1,limit:20,userGroupName:void 0},h={name:"ResearcherGroup",components:{Pagination:l["a"]},filters:{parseTime:o["d"]},data:function(){var e=this;return{quotaNamesValidator:function(t,r,n){Object(o["b"])(r)?n(new Error(e.$t("userGroup.quotaEmptyNotice"))):n()},list:[],listLoading:!0,listQuery:p,formTemplate:JSON.parse(JSON.stringify(m)),dialogFormVisible:!1,dialogStatus:"",textMap:{update:this.$t("userGroup.edit"),create:this.$t("userGroup.add")},userList:[],quotaList:[]}},created:function(){this.fetchGroupAndUsers(),this.getQuotas()},methods:{fetchGroupAndUsers:function(){var e=this,t={page:1,limit:-1};this.listLoading=!0,s.a.all([Object(d["c"])(this.listQuery),Object(c["f"])(t)]).then(s.a.spread((function(t,r){1e4===t.code&&(e.list=t.data.items||[]),1e4===r.code&&(e.userList=r.data.items||[])}))).catch((function(t){e.$notify({title:e.$t("user.retry"),message:e.$t("user.getDataFailed")+","+t,type:"error",duration:3e3})})).finally((function(){e.listLoading=!1}))},fetchData:function(){var e=this;this.listLoading=!0,Object(d["c"])(this.listQuery).then((function(t){e.list=t.data.items||[]})).catch((function(t){e.$notify({title:e.$t("user.refresh"),message:e.$t("user.getUserFailed")+","+t,type:"error",duration:2e3})})).finally((function(){e.listLoading=!1}))},refresh:function(){this.listQuery=p,this.userList=[],this.fetchGroupAndUsers(),this.getQuotas()},getQuotas:function(){var e=this;Object(f["d"])().then((function(t){e.quotaList=Object(f["c"])(t.data)})).catch((function(t){e.$notify({title:e.$t("user.refresh"),message:e.$t("user.getQuotaTreeFailed")+","+t,type:"error",duration:2e3})}))},resetFormTemplate:function(e){Object(o["b"])(e)?this.formTemplate=JSON.parse(JSON.stringify(m)):this.formTemplate=e},filterQuotaNodeByGroup:function(e){var t=[];if(this.list){var r=this.list.filter((function(e){return e.spec&&e.spec.quotaNames})).map((function(e){return Object(i["a"])(e.spec.quotaNames)})).flat();t=e.filter((function(e){return r.indexOf(e)<0}))}return t},handleCreate:function(){var e=this;this.resetFormTemplate(),this.dialogStatus="create",this.dialogFormVisible=!0,this.$nextTick((function(){e.$refs["dataForm"].clearValidate()}))},createData:function(){var e=this;this.$refs["dataForm"].validate((function(t){t&&Object(d["a"])(e.formTemplate,e.formTemplate.userNames).then((function(t){null!==t&&1e4===t.code?(e.refresh(),e.$notify({title:e.$t("user.success"),message:e.$t("user.createSuccess"),type:"success",duration:2e3})):e.$notify({title:e.$t("user.refresh"),message:e.$t("user.createFailed")+","+t.data,type:"error",duration:2e3})})).catch((function(t){e.$notify({title:e.$t("user.refresh"),message:e.$t("user.createFailed")+","+t,type:"error",duration:2e3})})).finally((function(){e.dialogFormVisible=!1,e.listLoading=!1}))}))},getUsersByGroup:function(e,t){console.log("group ",e," userList:",t);var r=t.filter((function(t){return t.spec&&(t.spec.groups||[]).includes(e)})).map((function(e){return e.spec.userName}));return console.log("group "+e+" users:",r),r},updateData:function(){var e=this;this.$refs["dataForm"].validate((function(t){if(t){console.log("group to update:",e.formTemplate,e.userList);var r=e.formTemplate.userNames.filter((function(e,t,r){return r.indexOf(e)===t})),n=e.userList.filter((function(e){return e.spec&&r.indexOf(e.spec.userName)>=0}));Object(d["f"])(e.formTemplate,n).then((function(t){if(null!==t&&1e4===t.code)return e.refresh(),void e.$notify({title:e.$t("user.success"),message:e.$t("user.updateSuccess"),type:"success",duration:2e3});e.$notify({title:e.$t("user.refresh"),message:e.$t("user.updateFailed")+","+t.data,type:"error",duration:2e3})})).catch((function(t){e.$notify({title:e.$t("user.refresh"),message:e.$t("user.updateFailed")+","+t,type:"error",duration:2e3})})).finally((function(){e.dialogFormVisible=!1}))}else e.dialogFormVisible=!1}))},deleteData:function(){var e=this;this.$refs["dataForm"].validate((function(t){t?(console.log("group to delete:",e.formTemplate),Object(d["b"])(e.formTemplate).then((function(t){if(null!==t&&1e4===t.code)return e.refresh(),void e.$notify({title:e.$t("user.success"),message:e.$t("user.deleteSuccess"),type:"success",duration:2e3});e.$notify({title:e.$t("user.refresh"),message:e.$t("user.deleteFailed")+","+t.data,type:"error",duration:2e3})})).catch((function(t){e.$notify({title:e.$t("user.refresh"),message:e.$t("user.deleteFailed")+","+t,type:"error",duration:2e3})})).finally((function(){e.dialogFormVisible=!1}))):e.dialogFormVisible=!1}))},handleUpdate:function(e){var t=this,r=this.getUsersByGroup(e.metadata.name,this.userList);this.resetFormTemplate(Object(d["e"])(e,r)),this.dialogStatus="update",this.dialogFormVisible=!0,this.$nextTick((function(){t.$refs["dataForm"].clearValidate()}))},handleDelete:function(e){var t=this,r=this.getUsersByGroup(e.metadata.name,this.userList);this.resetFormTemplate(Object(d["e"])(e,r)),console.log("group to delete:",this.formTemplate),this.listLoading=!0,Object(d["b"])(this.formTemplate).then((function(e){if(null!==e&&1e4===e.code)return t.refresh(),void t.$notify({title:t.$t("user.success"),message:t.$t("user.deleteSuccess"),type:"success",duration:2e3});t.$notify({title:t.$t("user.refresh"),message:t.$t("user.deleteFailed")+","+e.data,type:"error",duration:2e3})})).catch((function(e){t.$notify({title:t.$t("user.refresh"),message:t.$t("user.deleteFailed")+","+e,type:"error",duration:2e3})})).finally((function(){t.listLoading=!1}))}}},g=h,b=(r("b1fe"),r("2877")),v=Object(b["a"])(g,n,a,!1,null,"f00fb2d8",null);t["default"]=v.exports},"6f53":function(e,t,r){var n=r("83ab"),a=r("df75"),i=r("fc6a"),o=r("d1e7").f,u=function(e){return function(t){var r,u=i(t),s=a(u),l=s.length,c=0,d=[];while(l>c)r=s[c++],n&&!o.call(u,r)||d.push(e?[r,u[r]]:u[r]);return d}};e.exports={entries:u(!0),values:u(!1)}},"7a30":function(e,t,r){},a15b:function(e,t,r){"use strict";var n=r("23e7"),a=r("44ad"),i=r("fc6a"),o=r("a640"),u=[].join,s=a!=Object,l=o("join",",");n({target:"Array",proto:!0,forced:s||!l},{join:function(e){return u.call(i(this),void 0===e?",":e)}})},a2bf:function(e,t,r){"use strict";var n=r("e8b5"),a=r("50c4"),i=r("0366"),o=function(e,t,r,u,s,l,c,d){var f,m=s,p=0,h=!!c&&i(c,d,3);while(p<u){if(p in r){if(f=h?h(r[p],p,t):r[p],l>0&&n(f))m=o(e,t,f,a(f.length),m,l-1)-1;else{if(m>=9007199254740991)throw TypeError("Exceed the acceptable array length");e[m]=f}m++}p++}return m};e.exports=o},a434:function(e,t,r){"use strict";var n=r("23e7"),a=r("23cb"),i=r("a691"),o=r("50c4"),u=r("7b0b"),s=r("65f0"),l=r("8418"),c=r("1dde"),d=r("ae40"),f=c("splice"),m=d("splice",{ACCESSORS:!0,0:0,1:2}),p=Math.max,h=Math.min,g=9007199254740991,b="Maximum allowed length exceeded";n({target:"Array",proto:!0,forced:!f||!m},{splice:function(e,t){var r,n,c,d,f,m,v=u(this),y=o(v.length),$=a(e,y),N=arguments.length;if(0===N?r=n=0:1===N?(r=0,n=y-$):(r=N-2,n=h(p(i(t),0),y-$)),y+r-n>g)throw TypeError(b);for(c=s(v,n),d=0;d<n;d++)f=$+d,f in v&&l(c,d,v[f]);if(c.length=n,r<n){for(d=$;d<y-n;d++)f=d+n,m=d+r,f in v?v[m]=v[f]:delete v[m];for(d=y;d>y-n+r;d--)delete v[d-1]}else if(r>n)for(d=y-n;d>$;d--)f=d+n-1,m=d+r-1,f in v?v[m]=v[f]:delete v[m];for(d=0;d<r;d++)v[d+$]=arguments[d+2];return v.length=y-n+r,c}})},b1fe:function(e,t,r){"use strict";r("3872")},b85c:function(e,t,r){"use strict";r.d(t,"a",(function(){return a}));r("a4d3"),r("e01a"),r("d3b7"),r("d28b"),r("3ca3"),r("ddb0");var n=r("06c5");function a(e,t){var r="undefined"!==typeof Symbol&&e[Symbol.iterator]||e["@@iterator"];if(!r){if(Array.isArray(e)||(r=Object(n["a"])(e))||t&&e&&"number"===typeof e.length){r&&(e=r);var a=0,i=function(){};return{s:i,n:function(){return a>=e.length?{done:!0}:{done:!1,value:e[a++]}},e:function(e){throw e},f:i}}throw new TypeError("Invalid attempt to iterate non-iterable instance.\nIn order to be iterable, non-array objects must have a [Symbol.iterator]() method.")}var o,u=!0,s=!1;return{s:function(){r=r.call(e)},n:function(){var e=r.next();return u=e.done,e},e:function(e){s=!0,o=e},f:function(){try{u||null==r["return"]||r["return"]()}finally{if(s)throw o}}}}},c740:function(e,t,r){"use strict";var n=r("23e7"),a=r("b727").findIndex,i=r("44d2"),o=r("ae40"),u="findIndex",s=!0,l=o(u);u in[]&&Array(1)[u]((function(){s=!1})),n({target:"Array",proto:!0,forced:s||!l},{findIndex:function(e){return a(this,e,arguments.length>1?arguments[1]:void 0)}}),i(u)},d3a2:function(e,t,r){"use strict";r.d(t,"c",(function(){return m})),r.d(t,"e",(function(){return p})),r.d(t,"g",(function(){return h})),r.d(t,"f",(function(){return g})),r.d(t,"d",(function(){return b})),r.d(t,"b",(function(){return v})),r.d(t,"h",(function(){return y})),r.d(t,"a",(function(){return $}));r("d81d"),r("4ec9"),r("d3b7"),r("3ca3"),r("ddb0"),r("159b"),r("b0c0");var n=r("b775"),a=r("ed08"),i=r("bc3a"),o=r.n(i),u="kubeai-admin-clusterrole",s="kubeai-researcher-role",l="kubeai-researcher-clusterrole",c="admin";function d(e){function t(e,t){return{roleName:e,namespace:t}}e=f(e);var r=[];Object(a["b"])(e.clusterRoles)||(r=e.clusterRoles.map((function(e){return t(e,null)})),r=Object(a["e"])(r,["roleName"]));var n=[];if(!Object(a["b"])(e.roles)&&!Object(a["b"])(e.roleNamespaces)){for(var i=0;i<e.roles.length;i++)for(var o=e.roles[i],u=0;u<e.roleNamespaces.length;u++){var s=e.roleNamespaces[u];n.push({namespace:s,roleName:o})}n=Object(a["e"])(n,["namespace","roleName"])}var l={metadata:{name:e.uid},spec:{userName:e.userName,apiRoles:e.apiRoles.constructor===Array?e.apiRoles:[e.apiRoles],groups:e.groups||[],uid:e.uid,aliuid:e.aliuid,k8sServiceAccount:{roleBindings:n,clusterRoleBindings:r}}};return l}function f(e){var t=[],r=[];return r.push(s),t.push(l),e.apiRoles.indexOf(c)>=0&&(t.push(u),r.push(s)),e.roles=r,e.clusterRoles=t,e}function m(e){var t,r,n=e.spec,i=e.metadata;function o(e){var t=[];if(Object(a["b"])(e))return t;t=new Map;for(var r=0;r<e.length;r++){var n=e[r].namespace;t.has(n)||t.set(n,[]),t.get(n).push(e[r].roleName)}var i=[];return t.forEach((function(e,t){return i.push({namespace:t,roleNames:e})})),i}Object(a["b"])(n.k8sServiceAccount)||Object(a["b"])(n.k8sServiceAccount.clusterRoleBindings)||(t=n.k8sServiceAccount.clusterRoleBindings.map((function(e){return e.roleName}))),Object(a["b"])(n.k8sServiceAccount)||Object(a["b"])(n.k8sServiceAccount.roleBindings)||(r=o(n.k8sServiceAccount.roleBindings));var u={userName:n.userName,uid:i.name,aliuid:n.aliuid,apiRoles:n.apiRoles,clusterRoles:t,groups:n.groups||[],roles:r,createTime:i.creationTimestamp};return u}function p(){return Object(n["a"])({url:"/user/list/ramUsers",method:"get"})}function h(e){var t={userId:e.uid};return o()({url:"/researcher/getBearerToken",method:"GET",params:t})}function g(e){return Object(n["a"])({url:"/researcher/list",method:"get",params:e})}function b(e){var t={userId:e.uid,namespace:e.namespace};return o()({url:"/researcher/download/kubeconfig",method:"GET",responseType:"blob",params:t})}function v(e){var t=d(e);return Object(n["a"])({url:"/researcher/delete",method:"put",data:t})}function y(e){var t=d(e);return Object(n["a"])({url:"/researcher/update",method:"put",data:t})}function $(e){var t=d(e);return Object(n["a"])({url:"/researcher/create",method:"post",data:t})}}}]);