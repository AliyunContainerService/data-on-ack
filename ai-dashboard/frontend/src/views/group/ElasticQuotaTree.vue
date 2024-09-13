<template>
  <div class="app-container">
    <div class="filter-container">
      <el-input
        v-model="listQuery.name"
        :placeholder="$t('quota.name')"
        style="width: 200px"
        class="filter-item"
      />
      <el-button
        :disabled="tableData===undefined"
        class="filter-item"
        style="margin-left: 10px"
        type="primary"
        icon="el-icon-search"
        @click="handleFilter"
      >
        {{ $t('quota.search') }}
      </el-button>
      <el-button
        :disabled="isOnlyRootNode()"
        class="filter-item"
        style="margin-left: 10px"
        type="primary"
        icon="el-icon-edit"
        @click="appendResourceType"
      >
        {{ $t('quota.changeResourceType') }}
      </el-button>
      <el-button
        class="fr"
        style="margin-left: 10px"
        type="default"
        icon="el-icon-refresh-left"
        @click="getList"
      >
        {{ $t('job.refresh') }}
      </el-button>
    </div>

    <div class="custom-table-container">
      <div class="block">
        <el-table
          ref="table"
          :data="tableData"
          :row-key="getTableRowKey"
          border
          cell-class-name="quota-group-table"
          :indent="36"
          :tree-props="{children: 'children', hasChildren: 'hasChildren'}"
          highlight-current-row
          :expand-row-keys="expandRows"
          default-expand-all
        >
          <el-table-column
            :label="$t('quota.name')"
            prop="name"
            width="300"
            fixed
          />
          <el-table-column
            :label="$t('quota.namespace')"
            width="150"
          >
            <template slot-scope="scope">
              <span v-for="n in scope.row.namespaces || []" :key="n">{{ n }}<br></span>
            </template>
          </el-table-column>
          <el-table-column
            v-for="(item, index) in minColumns"
            :key="index"
            :label="item"
            min-width="100"
            show-overflow-tooltip
          >
            <template slot-scope="scope">
              <span style="color: #ccc">[&nbsp;</span>
              <span>{{ scope.row.min[item] }}</span>
              <span>&nbsp;,&nbsp;</span>
              <span>{{ scope.row.max[item] }}</span>
              <span style="color: #ccc">&nbsp;]</span>
            </template>
          </el-table-column>
          <el-table-column
            align="right"
            fixed="right"
            min-width="150"
            :label="$t('quota.operator')"
          >
            <template slot-scope="scope">
              <el-button v-if="canAddNode(scope.row)" type="text" @click="() => append(scope.row)"> {{ $t('quota.add') }} </el-button>
              <el-button v-if="canUpdate(scope.row)" type="text" @click="() => update(scope.row)"> {{ $t('quota.edit') }} </el-button>
              <el-button v-if="canDeleteNode(scope.row)" type="text" @click="() => remove(scope.row)"> {{ $t('quota.delete') }} </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <el-dialog :title="getDialogueTitle(dialogStatus)" :visible.sync="dialogFormVisible">
        <el-form
          ref="dataForm"
          :model="curData"
          label-position="left"
          label-width="0px"
          style="margin-left: 20px"
        >
          <el-form-item prop="name" label-width="120px" :label="$t('quota.name')" :rules="{required: true, validator: quotaNameValidate, trigger: 'blur'}">
            <el-input v-model="curData.name" :placeholder="getFormatExample('name')" :disabled="isDisableEditName(dialogStatus)" />
          </el-form-item>
          <el-form-item v-if="canChangeNamespace(curData, dialogStatus)" label-width="120px" :label="$t('quota.namespace')" prop="namespaces">
            <el-select v-model="curData.namespaces" :placeholder="$t('quota.placeholder.namespace')" :multiple="isMultiple()" filterable>
              <el-option
                v-for="(item, id) in namespaceList"
                :key="'namespaceitem'+id"
                :label="item"
                :value="item"
              />
            </el-select>
          </el-form-item>
          <el-card v-if="curData.quotaConfigs!==undefined">
            <div slot="header">
              <el-row>
                <el-col :span="10">{{ $t('quota.resourceType') }}</el-col>
                <el-col :span="5">{{ $t('quota.min') }}</el-col>
                <el-col :span="5">{{ $t('quota.max') }}</el-col>
                <el-col :span="2">{{ $t('quota.comment') }}</el-col>
                <el-col v-if="dialogStatus==='changeResourceType'" :span="1">{{ $t('quota.add') }}</el-col>
                <el-col v-if="dialogStatus==='changeResourceType'" :span="1"> <i class="el-icon-circle-plus-outline" style="color:green;" @click="appendOtherResource" /></el-col>
                <el-col v-if="dialogStatus!=='changeResourceType'" :span="2" />
              </el-row>
            </div>
            <div
              v-for="(quotaConfig, index) in curData.quotaConfigs"
              :key="'quotaConfig'+index"
            >
              <el-row type="flex" class="row-bg" style="margin-bottom:7px" justify="space-between">
                <el-col :span="10">
                  <el-form-item :prop="'quotaConfigs.'+index+'.type'" :rules="{required: true, validator: resourceTypeValidator, trigger: 'blur'}">
                    <el-input v-if="isDefaultResourceType(quotaConfig.type, index)" :placeholder="quotaConfig.type" :disabled="!isResourceTypeEditable(dialogStatus, quotaConfig.type, index)" />
                    <el-input v-else v-model="quotaConfig.type" :placeholder="quotaConfig.type" :disabled="dialogStatus!=='changeResourceType'" />
                  </el-form-item>
                </el-col>
                <el-col :span="5" style="margin-left:10px">
                  <el-form-item :prop="'quotaConfigs.'+index+'.min'" :rules="{required:false, validator: quotaFormatValidator, trigger:'blur'}">
                    <el-input v-model="quotaConfig.min" :placeholder="getFormatExample(quotaConfig.type)" />
                  </el-form-item>
                </el-col>
                <el-col :span="5" style="margin-left:10px;">
                  <el-form-item :prop="'quotaConfigs.'+index+'.max'" :rules="{required:false, validator: quotaFormatValidator, trigger:'blur'}">
                    <el-input v-model="quotaConfig.max" :placeholder="getFormatExample(quotaConfig.type)" />
                  </el-form-item>
                </el-col>
                <el-col v-if="!isEmptyWrapper(getDefaultQuotaComments()[quotaConfig.type])" :span="2" style="margin-left:10px; margin-top:14px">
                  {{ !isEmptyWrapper(getDefaultQuotaComments()[quotaConfig.type]['shortComments'])? getDefaultQuotaComments()[quotaConfig.type]['shortComments'] : '' }}
                  <i
                    v-if="!isEmptyWrapper(getDefaultQuotaComments()[quotaConfig.type]['helpUrl'])"
                    class="el-icon-info"
                    :title="getDefaultQuotaComments()[quotaConfig.type]['longComments']"
                    @click="clickComments(quotaConfig.type)"
                  /></el-col>
                <el-col v-else :span="2" style="margin-left:10px; margin-top:14px" />
                <el-col v-if="dialogStatus==='changeResourceType'" :span="1" style="margin-left:10px" />
                <el-col v-if="isResourceTypeEditable(dialogStatus, quotaConfig.type, index)" :span="1"> <i class="el-icon-delete" style="color:red; margin-top: 14px" @click="removeOtherResource(index)" /></el-col>
                <el-col v-else :span="1" />
                <el-col v-if="dialogStatus!=='changeResourceType'" :span="2" />
              </el-row>
            </div>
          </el-card>
        </el-form>
        <div slot="footer" class="dialog-footer">
          <el-button @click="dialogFormVisible = false"> {{ $t('quota.cancel') }} </el-button>
          <el-button type="primary" @click="saveClickRouter(dialogStatus)"> {{ $t('quota.save') }} </el-button>
        </div>
      </el-dialog>

    </div>
  </div>
</template>

<script>
import { fetchElasticQuotaTree,
  parseElasticQuotaTree,
  serializeElasticQuotaTree,
  updateElasticeQuotaTree,
  serializeMinMax,
  parseSearchData,
  doRemovedTableData } from '@/api/group'
import { fetchNamespaceList } from '@/api/k8s'
import { isEmpty, isArray, isListEqual } from '@/utils'
import { getClusterInfo } from '@/utils/auth'
import { IsK8SVersionSatisfied } from '@/api/group'

export default {
  data() {
    const defaultCurData = {
      name: '',
      prefix: '',
      namespaces: [],
      min: {},
      max: {},
      children: []
    }

    return {
      validateQuotaLogic(resourceQuota) {
        const min = resourceQuota.min
        const max = resourceQuota.max
        if (!isEmpty(min) && !isEmpty(max)) {
          try {
            let isNumberRelationIlleagal = false
            if (min === 'N/A' && max !== 'N/A') {
              isNumberRelationIlleagal = true
            }
            if (Number(resourceQuota.min) > Number(resourceQuota.max)) {
              isNumberRelationIlleagal = true
            }
            if (isNumberRelationIlleagal) {
              this.$notify({
                title: this.$t('quota.validator.minMaxRelationError'),
                dangerouslyUseHTMLString: true,
                message: `${resourceQuota.type} min>max min:${min} max:${max}`,
                type: 'error',
                duration: 2000
              })
              return false
            }
          } catch {
            return true
          }
        }
        return true
      },

      validateQuotaFormat(quotaName, quota) {
        var formatExp = /(^\d+$)|(^N\/A$)/g
        var defaultQuotaComments = this.getDefaultQuotaComments()
        if (defaultQuotaComments[quotaName] !== undefined && defaultQuotaComments[quotaName].format !== undefined) {
          formatExp = defaultQuotaComments[quotaName].format
        }
        if (isEmpty(quota)) {
          this.$notify({
            title: this.$t('quota.validator.formatError'),
            message: 'Name:' + quotaName + ' empty',
            dangerouslyUseHTMLString: true,
            type: 'error',
            duration: 2000
          })
          return false
        }
        var matchedQuota = quota.match(formatExp)
        if (matchedQuota === null) {
          this.$notify({
            title: this.$t('quota.validator.formatError'),
            dangerouslyUseHTMLString: true,
            message: 'Name:' + quotaName + ' quota:' + quota + ', should be:' + formatExp,
            type: 'error',
            duration: 2000
          })
          return false
        }
        return true
      },

      quotaNameValidate: (rule, value, callback) => {
        if (this.isDisableEditName(this.dialogStatus)) {
          callback()
          return
        }
        // check char set
        var quotaName = rule.field.split('.')[0]
        if (!this.validateQuotaFormat(quotaName, value)) {
          if (!isEmpty(this.getDefaultQuotaComments()[quotaName]) && !isEmpty(this.getDefaultQuotaComments()[quotaName].formatExample)) {
            callback(new Error(this.getDefaultQuotaComments()[quotaName].formatExample))
            return
          }
          callback(new Error(this.$t('quota.validator.formatError')))
          return
        }

        const prefix = this.editRow.prefix || ''
        const roots = this.$refs.table.tableData
        const p = prefix.split('.')

        let comparedChildren = p.reduce((pre, cur) => {
          const cIndex = pre.find(d => d.name === cur)
          return cIndex ? cIndex.children : []
        }, roots)

        if (prefix === '') {
          comparedChildren = roots
        }

        let repeatedName = (comparedChildren || []).filter(d => d.name === value)
        if (this.dialogStatus === 'addQuota') {
          const curAdd = (comparedChildren || []).find(d => d.name === this.editRowName)
          repeatedName = curAdd ? (curAdd.children || []).filter(d => d.name === value) : []
        }
        if (this.dialogStatus === 'editQuota' && this.editRowName) {
          repeatedName = (comparedChildren || []).filter(d => d.name === value && d.name !== this.editRowName)
        }
        if (repeatedName.length > 0) {
          callback(new Error(this.$t('quota.validator.nameRepeat')))
          return
        }
        callback()
      },

      resourceTypeValidator: (rule, value, callback) => {
        const resourceIdx = Number(rule.field.split('.')[1])
        if (!this.validateQuotaFormat('resourceType', value)) {
          callback(new Error(this.$t('quota.validator.formatError') + ':' + this.getDefaultQuotaComments()['resourceType'].formatExample))
          return
        }

        if (this.resourceTypes.indexOf(value) >= 0 && this.resourceTypes.indexOf(value) !== resourceIdx) {
          callback(new Error(this.$t('quota.validator.resourceTypeNotUnique')))
          return
        }
        callback()
      },

      quotaFormatValidator: (rule, value, callback) => {
        const resourceMemberName = rule.field.split('.')[0]
        const resourceIdx = Number(rule.field.split('.')[1])
        var resourceType = ''
        var checkingConfigs = this.curData.quotaConfigs
        if (resourceMemberName === 'quotaConfigs') {
          resourceType = checkingConfigs[resourceIdx].type
        }
        if (!this.validateQuotaFormat(resourceType, value)) {
          if (!isEmpty(this.getDefaultQuotaComments()[resourceType]) && !isEmpty(this.getDefaultQuotaComments()[resourceType].formatExample)) {
            callback(new Error(this.getDefaultQuotaComments()[resourceType].formatExample))
            return
          }
          callback(new Error(this.$t('quota.validator.mustBeNumber')))
          return
        }
        if (!this.validateQuotaLogic(checkingConfigs[resourceIdx])) {
          callback(new Error(this.$t('quota.validator.minMaxRelationError')))
          return
        }
        callback()
      },
      k8sElasticQuotaTree: undefined,
      treeData: [],
      tableData: [],
      listQuery: { name: '' },
      namespaceList: [],
      resourceTypes: ['cpu', 'memory', 'nvidia.com/gpu', 'aliyun.com/gpu', 'aliyun.com/gpu-mem'],
      defaultResourceTypes: ['cpu', 'memory', 'nvidia.com/gpu', 'aliyun.com/gpu', 'aliyun.com/gpu-mem'],
      allNamespaceList: [],
      dialogFormVisible: false,
      dialogStatus: '',
      curNode: undefined,
      curData: JSON.parse(JSON.stringify(defaultCurData)),
      TreeAction: {
        CreateTree: 'createTree',
        AddNode: 'addNode',
        DeleteNode: 'deleteNode',
        UpdateNode: 'updateNode',
        UpdateResourceType: 'updateResourceType'
      },
      defaultCurData: defaultCurData,
      defaultProps: {
        children: 'children',
        label: 'name',
        id: 'name'
      },
      expandRows: [],
      minColumns: []
    }
  },

  created() {
    this.getList()
    // Warn if overriding existing method
    if (!Array.prototype.equals) {
      console.warn("Overriding existing Array.prototype.equals. Possible causes: New API defines the method, there's a framework conflict or you've got double inclusions in your code.")
      // attach the .equals method to Array's prototype to call it on any array
      /*eslint no-extend-native: ["error", { "exceptions": ["Array"] }]*/
      Array.prototype.equals = isListEqual
      Object.defineProperty(Array.prototype, 'equals', { enumerable: false })
    }
  },

  methods: {
    getDefaultQuotaComments() {
      return {
        'name': {
          longComments: '',
          formatExample: '请输入名称（可包含数字、字母、（_），不能以（_）开头）',
          format: /^([a-zA-Z]{1})([_a-zA-Z0-9]{0,64})$/g,
          shortComments: 'name'
        },
        'resourceType': {
          longComments: '',
          formatExample: '资源类型只能是以字母开头，包含字母、数字、（-）、（_）、（.）、（/）的字符串',
          format: /^([a-zA-Z]{1})([\.\/\-_.a-zA-Z0-9]{0,64})$/g,
          shortComments: 'resourceType'
        },
        'cpu': {
          longComments: '',
          formatExample: '1,0.1,100m',
          format: /^((\d+)\.?(\d{0,})|N\/A)(m{0,1})$/g,
          shortComments: 'core',
          helpUrl: 'https://kubernetes.io/zh/docs/concepts/configuration/manage-resources-containers/#meaning-of-cpu'
        },
        'memory': {
          longComments: '',
          format: /^((\d+)\.?(\d{0,})|N\/A)([EPTGMK]{0,1}[i]{0,1})$/g,
          formatExample: '128,129M,123Mi',
          shortComments: 'M',
          helpUrl: 'https://kubernetes.io/zh/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory'
        },
        'aliyun.com/gpu-mem': {
          longComments: 'gpu memory',
          format: /^(\d+|N\/A)([G]{0,1}[i]{0,1})$/g,
          shortComments: 'G',
          helpUrl: 'https://help.aliyun.com/document_detail/191152.html?spm=a2c4g.11186623.6.686.54691effXkB41b'
        },
        'nvidia.com/gpu': {
          longComments: 'gpu',
          formatExp: /(^\d+$)|(^N\/A$)/g,
          shortComments: this.$t('quota.gpuCard'),
          helpUrl: undefined
        },
        'aliyun.com/gpu': {
          longComments: 'gpu topology',
          formatExp: /(^\d+$)|(^N\/A$)/g,
          shortComments: this.$t('quota.gpuCard'),
          helpUrl: 'https://help.aliyun.com/document_detail/190482.html'
        },
        'aliyun.com/gpu-core.percentage': {
          longComments: 'gpu core percentage',
          formatExp: /(^\d+$)|(^N\/A$)/g,
          shortComments: this.$t('quota.gpuCard'),
          helpUrl: 'https://help.aliyun.com/document_detail/424668.html'
        }
      }
    },

    getDialogueTitle(dialogStatus) {
      return this.$t('quota.' + dialogStatus)
      // create: this.$t('quota.addQuota'),
      // addResourceType: this.$t('quota.changeResourceType')
      // this.$t()
    },
    filterNamespace(allNamespaceList) {
      if (this.k8sElasticQuotaTree === undefined || this.k8sElasticQuotaTree.spec === undefined) {
        return allNamespaceList.map(x => x.name)
      }
      var quotaManagedNamespaceList = parseElasticQuotaTree(this.k8sElasticQuotaTree.spec.root)[2].root
      return allNamespaceList.filter(x => quotaManagedNamespaceList.indexOf(x.name) < 0).map(x => x.name)
    },

    getFormatExample(resourceType) {
      var defaultExample = this.$t('quota.validator.mustBeNumber')
      var defaultQuotaComments = this.getDefaultQuotaComments()
      if (defaultQuotaComments[resourceType] === undefined) {
        return defaultExample
      }
      if (isEmpty(defaultQuotaComments[resourceType].formatExample)) {
        return defaultExample
      }
      return defaultQuotaComments[resourceType].formatExample
    },

    isMultiple() {
      const clusterInfo = getClusterInfo()
      return IsK8SVersionSatisfied(clusterInfo)
    },

    getNamespace() {
      fetchNamespaceList().then((response) => {
        this.allNamespaceList = [...response.data]
        this.namespaceList = this.filterNamespace(this.allNamespaceList)
      }).catch((error) => {
        this.$notify({
          title: this.$t('quota.retry'),
          dangerouslyUseHTMLString: true,
          message: this.$t('quota.exception.getNamespace') + 'Error:' + error,
          type: 'error',
          duration: 2000
        })
      })
    },

    getList() {
      this.listLoading = true
      fetchElasticQuotaTree().then((response) => {
        this.listQuery.name = ''
        this.treeData = []
        var rootNode = JSON.parse(JSON.stringify(this.defaultCurData))
        rootNode.name = 'root'
        this.tableData = isEmpty(response.data) ? [rootNode] : [response.data?.spec?.root]

        if (this.tableData[0] && this.tableData[0].min) {
          this.minColumns = Object.keys(this.tableData[0].min || {})
        }

        this.k8sElasticQuotaTree = response.data
        if (this.k8sElasticQuotaTree === undefined) {
          this.k8sElasticQuotaTree = { spec: { root: {}}}
        }
        if (isEmpty(response.data)) {
          this.treeData.push(rootNode)
          return
        }
        var [parsedTreeData, parsedResourceTypes, parsedNamespaces] = parseElasticQuotaTree(this.k8sElasticQuotaTree.spec.root)
        if (isEmpty(parsedTreeData)) {
          this.treeData.push(rootNode)
          return
        }
        parsedNamespaces
        this.treeData.push(parsedTreeData)
        this.resourceTypes = [...this.defaultResourceTypes, ...parsedResourceTypes.root].filter((v, i, self) => self.indexOf(v) === i)
      }).catch((error) => {
        this.$notify({
          title: this.$t('quota.retry'),
          dangerouslyUseHTMLString: true,
          message: this.$t('quota.exception.getElasticQuotaTree') + ':' + error,
          type: 'error',
          duration: 2000
        })
      }).finally(() => {
        this.listLoading = false
      })
    },

    packKVObjectToString(maxObject, kkSpliter = ',', kvSpliter = ':') {
      var res = []
      for (const [key, value] of Object.entries(maxObject)) {
        res.push([key, value].join(kvSpliter))
      }
      return res.join(kkSpliter)
    },

    handleFilter() {
      const roots = this.$refs.table.tableData
      const prefix = parseSearchData('prefix', roots[0], this.listQuery.name)
      this.expandRows = (prefix || '').split('.')

      const currentRow = parseSearchData('currentRow', roots[0], this.listQuery.name)
      this.$refs.table.setCurrentRow(currentRow)

      this.$nextTick(function() {
        const scrollToTop = document.querySelector('.current-row')
        if (currentRow && scrollToTop) {
          scrollToTop.scrollIntoView({
            block: 'start',
            behavior: 'smooth'
          })
        }
      })
    },

    treeFilter(value, data) {
      if (!data || isEmpty(data.name)) {
        return false
      }
      if (!value) return true
      return data.name.indexOf(value) !== -1
    },

    appendOtherResource() {
      this.curData.quotaConfigs.push({
        type: '',
        min: '0',
        max: 'N/A'
      })
    },

    removeOtherResource(index) {
      this.curData.quotaConfigs.splice(index, 1)
    },

    updateTreeResourceType(treeRoot, newResources, resourceTypeMap) {
      function mergeMap(toMap, newResources, defaultValue, resourceTypeMap) {
        var resMap = {}
        newResources.forEach(x => {
          const oldKey = resourceTypeMap[x]
          var oldValue = toMap[x]
          if (!isEmpty(oldKey) && isEmpty(oldValue)) {
            oldValue = toMap[oldKey]
          }
          resMap[x] = oldValue || defaultValue
        })
        // map(x => [x, toMap[x] === undefined ? defaultValue : toMap[x]])
        // toMap = Object.fromEntries(new Map(newKVList))
        return resMap
      }
      if (isEmpty(treeRoot)) {
        return
      }
      if (!isEmpty(treeRoot.min)) {
        treeRoot.min = mergeMap(treeRoot.min, newResources, '0', resourceTypeMap)
      }

      if (!isEmpty(treeRoot.max)) {
        treeRoot.max = mergeMap(treeRoot.max, newResources, 'N/A', resourceTypeMap)
      }

      // this.genK8sQuotaConfig(treeRoot)
      if (!isEmpty(treeRoot.children)) {
        for (var i = 0; i < treeRoot.children.length; i++) {
          this.updateTreeResourceType(treeRoot.children[i], newResources, resourceTypeMap)
        }
      }
    },

    notifyDependResponse(response, successMsgKey, errorMsgKey) {
      if (response.code === 10000) {
        this.$notify({
          title: this.$t('quota.success'),
          message: this.$t(successMsgKey),
          type: 'success',
          duration: 2000
        })
      } else {
        this.$notify({
          title: this.$t('quota.error'),
          dangerouslyUseHTMLString: true,
          message: this.$t(errorMsgKey) + ':' + response.data,
          type: 'error',
          duration: 2000
        })
      }
    },

    doAppendResourceType() {
      this.$refs['dataForm'].validate((valid) => {
        if (!valid) {
          return
        }
        var oldResources = [...this.resourceTypes]
        var newResources = this.curData.quotaConfigs.map(x => x.type)
        var resourceNameMap = {}
        newResources.forEach((x, index) => { resourceNameMap[x] = index < oldResources.length ? oldResources[index] : undefined })
        // update resource types to tree
        if (!oldResources.equals(newResources)) {
          this.resourceTypes = [...newResources]
          this.updateTreeResourceType(this.tableData[0], newResources, resourceNameMap)
        }

        this.genK8sQuotaConfig(this.curData)
        Object.assign(this.editRow, {
          min: this.curData.min,
          max: this.curData.max,
          namespaces: isArray(this.curData.namespaces) ? this.curData.namespaces : [this.curData.namespaces]
        })

        const actionType = this.TreeAction.UpdateResourceType
        const curName = this.curData.name
        const data = Object.assign({}, this.k8sElasticQuotaTree, { spec: { root: this.tableData[0] }})
        updateElasticeQuotaTree(data, actionType, curName, curName, '').then((response) => {
          this.getList()
          this.notifyDependResponse(response, 'quota.ok.changeResourceType', 'quota.exception.changeResourceType')
        }).catch((error) => {
          this.getList()
          this.$notify({
            title: this.$t('quota.retry'),
            dangerouslyUseHTMLString: true,
            message: this.$t('quota.exception.changeResourceType') + ':' + error,
            type: 'error',
            duration: 2000
          })
        }).finally(() => {
          this.dialogFormVisible = false
        })
      })
    },

    appendResourceType() {
      this.dialogStatus = 'changeResourceType'
      var rootData = this.tableData[0]
      this.editRow = this.tableData[0]
      this.curData = this.resetCurDataTo(rootData)
      this.dialogFormVisible = true

      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
        this.listLoading = false
      })
    },

    udpateElasticeQuotaTree(treeData, k8sTree) {
      var res = serializeElasticQuotaTree(treeData[0], res)
      k8sTree.spec.root = res
      return k8sTree
    },

    getTableRowKey(row) {
      return (row.prefix || '') + row.name
    },

    update(data) {
      this.editRow = data
      this.editRowName = data.name
      this.dialogStatus = 'editQuota'
      this.curData = this.resetCurDataTo(data)
      if (this.allNamespaceList === undefined || this.allNamespaceList.length < 1) {
        this.getNamespace()
      }
      this.dialogFormVisible = true

      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
        this.listLoading = false
      })
    },

    doUpdate() {
      this.$refs['dataForm'].validate((valid) => {
        if (!valid) {
          return
        }

        this.genK8sQuotaConfig(this.curData)
        const oldNodeName = this.editRow.name
        const newNodeName = this.curData.name
        const nodePrefix = this.editRow.prefix || ''
        const action = this.tableData.length === 1 && isEmpty(this.tableData[0].children) ? 'createTree' : this.TreeAction.UpdateNode

        Object.assign(this.editRow, {
          name: this.curData.name,
          namespaces: isArray(this.curData.namespaces) ? this.curData.namespaces : [this.curData.namespaces],
          min: this.curData.min,
          max: this.curData.max
        })
        const data = Object.assign({}, this.k8sElasticQuotaTree, { spec: { root: this.tableData[0] }})
        updateElasticeQuotaTree(data, action, oldNodeName, newNodeName, nodePrefix).then((response) => {
          this.getList()
          this.notifyDependResponse(response, 'quota.ok.updateSuccess', 'quota.exception.updateQuota')
        }).catch((error) => {
          this.$notify({
            title: this.$t('quota.retry'),
            dangerouslyUseHTMLString: true,
            message: error,
            type: 'error',
            duration: 2000
          })
          this.getList()
        }).finally(() => {
          this.dialogFormVisible = false
        })
      })
    },

    isOnlyRootNode() {
      return this.treeData.length === 1 && isEmpty(this.treeData[0].children)
    },

    doAppend() {
      this.$refs['dataForm'].validate((valid) => {
        if (!valid) {
          return
        }

        var action = this.tableData.length === 1 && isEmpty(this.tableData[0].children) ? 'createTree' : 'addNode'
        var changingNodeName = this.curData.name
        var prefix = this.editRow.prefix ? [this.editRow.prefix, this.editRow.name].join('.') : this.editRow.name

        this.genK8sQuotaConfig(this.curData)
        var newChild = {
          name: changingNodeName,
          prefix,
          min: this.curData.min,
          max: this.curData.max,
          namespaces: isArray(this.curData.namespaces) ? this.curData.namespaces : [this.curData.namespaces]
        }
        this.editRow.children = (this.editRow.children || []).concat(newChild)
        var data = Object.assign({}, this.k8sElasticQuotaTree, { spec: { root: this.tableData[0] }})
        updateElasticeQuotaTree(data, action, changingNodeName, changingNodeName, prefix).then((response) => {
          // this.list.unshift(this.group)
          this.dialogFormVisible = false
          this.getList()
          this.notifyDependResponse(response, 'quota.ok.addSuccess', 'quota.exception.addQuota')
        }).catch((error) => {
          this.getList()
          this.$notify({
            title: this.$t('quota.retry'),
            dangerouslyUseHTMLString: true,
            message: error,
            type: 'error',
            duration: 2000
          })
        }).finally(() => {
          this.dialogFormVisible = false
        })
      })
    },
    append(data) {
      this.editRow = data
      this.editRowName = data.name
      this.dialogStatus = 'addQuota'
      this.curData = this.resetCurDataTo(this.defaultCurData, data)

      if (this.allNamespaceList === undefined || this.allNamespaceList.length < 1) {
        this.getNamespace()
      } else {
        this.namespaceList = this.filterNamespace(this.allNamespaceList)
      }
      this.dialogFormVisible = true

      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
        this.listLoading = false
      })
    },
    remove(rRata) {
      const { name, prefix } = rRata
      const tableData = doRemovedTableData(this.tableData[0], name, prefix)
      const data = Object.assign({}, this.k8sElasticQuotaTree, { spec: { root: tableData }})
      updateElasticeQuotaTree(data, this.TreeAction.DeleteNode, name, null, prefix).then((response) => {
        this.getList()
        this.notifyDependResponse(response, 'quota.ok.deleteSuccess', 'quota.exception.deleteQuota')
      }).catch((error) => {
        this.getList()
        this.$notify({
          title: this.$t('quota.retry'),
          dangerouslyUseHTMLString: true,
          message: this.$t('quota.exception.deleteQuota') + ':' + error,
          type: 'error',
          duration: 2000
        })
      })
    },
    clickComments(resourceName) {
      var comment = this.getDefaultQuotaComments()[resourceName]
      if (isEmpty(comment)) {
        return
      }
      var helpUrl = comment.helpUrl
      if (isEmpty(helpUrl)) {
        return
      }
      window.open(helpUrl, '_blank')
    },

    isLeaf(data) {
      return isEmpty(data.children) || data.children.length < 1
    },

    isDisableEditName(dialogStatus) {
      return dialogStatus === 'changeResourceType'
    },

    canUpdate(data) {
      if (data.name !== undefined) {
        return true
      }
      return false
    },

    canDeleteNode(data) {
      if (data.name === undefined || this.isOnlyRootNode()) {
        return false
      }
      if (this.isLeaf(data)) {
        if (this.isLeaf(this.tableData[0])) { // that's to say curNode is leaf and curNode is lastNode
          return false
        }
        if (data.namespaces === undefined || data.namespaces.length < 1) {
          return true
        }
        // TODO request to check whetcher cur node pod is empty
      }
      return false
    },

    canAddNode(data) {
      if (this.isLeaf(data)) {
        if (data.namespaces === undefined || data.namespaces.length < 1) {
          return true
        }
        // TODO request to check whetcher cur node pod is empty
        return false
      }
      return true
    },

    canChangeNamespace(data, dialogStatus) {
      if (this.isOnlyRootNode() && dialogStatus !== 'addQuota') {
        return false
      }
      return this.isLeaf(data)
    },

    isEmptyWrapper(v) {
      return isEmpty(v)
    },

    genK8sQuotaConfig(curData) {
      curData.min = Object.fromEntries(new Map(curData.quotaConfigs.map(x => [x.type, x.min])))
      curData.max = Object.fromEntries(new Map(curData.quotaConfigs.map(x => [x.type, x.max])))
      if (!isEmpty(curData.min.memory)) {
        serializeMinMax(curData.min)
        serializeMinMax(curData.max)
      }
      return curData
    },

    resetCurDataTo(referData, parentData) {
      var curData = JSON.parse(JSON.stringify(referData))

      if (undefined === curData.quotaConfigs) {
        curData.quotaConfigs = []
      }
      if (undefined === curData.otherResources) {
        curData.otherResources = []
      }
      if (curData.quotaConfigs.length < 1) {
        for (const o of this.resourceTypes.entries()) {
          var rtype = o[1]
          var min = !isEmpty(curData.min) && curData.min[rtype] ? curData.min[rtype] : '0'
          var max = !isEmpty(curData.max) && curData.max[rtype] ? curData.max[rtype] : (parentData && parentData.max && parentData.max[rtype] || 'N/A')
          curData.quotaConfigs.push({
            type: rtype,
            min: min,
            max: max
          })
        }
      }
      return curData
    },

    saveClickRouter(dialogStatus) {
      if (dialogStatus === 'addQuota') {
        this.doAppend()
      } else if (dialogStatus === 'changeResourceType') {
        this.doAppendResourceType()
      } else {
        this.doUpdate()
      }
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },

    isDefaultResourceType(curQuotaType, index) {
      return this.defaultResourceTypes.indexOf(curQuotaType) >= 0 && index < this.defaultResourceTypes.length
    },

    isResourceTypeEditable(dialogStatus, resourceType, index) {
      const isDefault = this.isDefaultResourceType(resourceType, index)
      return dialogStatus === 'changeResourceType' && !isDefault
    },

    isNewResourceType(curQuotaConfigIndex) {
      // we depend resource type length to check whether cur resource type is newly added
      return !(curQuotaConfigIndex < this.resourceTypes.length)
    }
  }
}
</script>

<style>
.custom-table-container {
  margin-top: 10px;
}

.table-cell-line {
  line-height: 1.3;
}

.table-cell-label {
  color: #999;
  margin-right: 10px;
}

.el-select {
  display: block;
}

.quota-group-table .cell i {
  font-weight: 700;
  font-size: 14px;
  color: #409EFF;
}
</style>
