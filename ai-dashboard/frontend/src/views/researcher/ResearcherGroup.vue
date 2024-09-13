<template>
  <div class="app-container">
    <div class="filter-container">
      <el-input
        v-model="listQuery.userGroupName"
        :placeholder="$t('userGroup.name')"
        style="width: 200px"
        class="filter-item"
        @keyup.enter.native="fetchData"
      />
      <el-button
        class="filter-item"
        style="margin-left: 10px"
        type="primary"
        icon="el-icon-search"
        @click="fetchData"
      >
        {{ $t('user.search') }}
      </el-button>
      <el-button
        class="filter-item"
        style="margin-left: 10px"
        type="primary"
        icon="el-icon-edit"
        @click="handleCreate"
      >
        {{ $t('user.add') }}
      </el-button>
      <el-button
        class="fr"
        style="margin-left: 10px"
        type="default"
        icon="el-icon-refresh-left"
        @click="refresh"
      >
        {{ $t('userGroup.refresh') }}
      </el-button>
    </div>

    <el-table
      v-loading="listLoading"
      :data="list"
      element-loading-text="Loading"
      border
      fit
      highlight-current-row
      style="margin-top: 20px"
      row-key="id"
    >
      <el-table-column
        prop="spec.groupName"
        :label="$t('userGroup.name')"
      />
      <el-table-column
        :label="$t('user.quota')"
      >
        <template slot-scope="scope">
          <span v-for="n in scope.row.spec.quotaNames" :key="n" class="mr2">
            {{ n }}
          </span>
        </template>
      </el-table-column>
      <el-table-column
        :label="$t('userGroup.user')"
      >
        <template slot-scope="scope">
          <div v-for="n in getUsersByGroup(scope.row.metadata.name, userList)" :key="n" class="mr2">
            {{ n }}
          </div>
        </template>
      </el-table-column>
      <el-table-column
        prop="metadata.creationTimestamp"
        :label="$t('user.createTime')"
        width="180"
      />
      <el-table-column
        :label="$t('user.operator')"
        align="center"
        width="160"
        class-name="small-padding fixed-width"
      >
        <template slot-scope="scope">
          <el-button
            type="primary"
            size="mini"
            @click="handleUpdate(scope.row)"
          >
            {{ $t('user.edit') }}
          </el-button>
          <el-button
            size="mini"
            type="danger"
            @click="handleDelete(scope.row, scope.$index)"
          >
            {{ $t('user.delete') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <pagination
      v-show="list.length > 0"
      :total="list.length"
      :page.sync="listQuery.page"
      :limit.sync="listQuery.limit"
      @pagination="fetchData"
    />
    <el-dialog :title="textMap[dialogStatus]" :visible.sync="dialogFormVisible">
      <el-form
        ref="dataForm"
        :model="formTemplate"
        label-position="left"
        label-width="120px"
        style="margin-left: 20px"
      >
        <el-form-item
          :label="$t('userGroup.name')"
          prop="name"
          :rules="[
            {required: true, message: $t('userGroup.nameEmptyNotice'), trigger: 'blur'},
            {min: 1, max: 63, message: $t('userGroup.invalidNameLength'), trigger: 'blur'},
            {
              message: $t('userGroup.invalidNamePattern'),
              trigger: 'blur',
              type: 'string',
              pattern: /^[0-9a-zA-Z][0-9a-zA-Z-]*$/,
            }]"
        >
          <el-input
            v-model="formTemplate.name"
            :placeholder="$t('userGroup.namePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="$t('userGroup.quotaNode')" prop="quotaNames" :rules="{required: true, validator:quotaNamesValidator, message: $t('userGroup.quotaEmptyNotice'), trigger: 'blur'}">
          <el-input v-if="dialogStatus==='update'" :placeholder="formTemplate.quotaNames" :disabled="true" />
          <el-select
            v-else
            v-model="formTemplate.quotaNames"
            :placeholder="$t('userGroup.quotaNames')"
          >
            <el-option
              v-for="item in filterQuotaNodeByGroup(quotaList)"
              :key="item"
              :label="item"
              :value="item"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('userGroup.user')">
          <el-select
            v-model="formTemplate.userNames"
            :placeholder="$t('userGroup.userNames')"
            multiple
          >
            <el-option
              v-for="item in userList"
              :key="item.spec.userId"
              :label="item.spec.userName"
              :value="item.spec.userName"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <div slot="footer" class="dialog-footer">
        <el-button @click="dialogFormVisible = false"> {{ $t('user.cancel') }} </el-button>
        <el-button
          type="primary"
          @click="dialogStatus === 'create' ? createData() : updateData()"
        >
          {{ $t('user.save') }}
        </el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>

import { parseTime, isEmpty } from '@/utils'
import axios from 'axios'
import Pagination from '@/components/Pagination' // secondary package based on el-pagination
import { fetchResearcherList } from '@/api/researcher'
import { fetchUserGroup, createUserGroup, updateUserGroup, deleteUserGroup, fromK8sUserGroup } from '@/api/userGroup'
import { fetchElasticQuotaTree, extractLeafNamesFromQuotaTree } from '@/api/group'
export const constFormTemplate = {
  name: '',
  userNames: [],
  quotaNames: ''
}
const rawQuery = {
  page: 1,
  limit: 20,
  userGroupName: undefined
}

export default {
  name: 'ResearcherGroup',
  components: {
    Pagination
  },
  filters: {
    parseTime: parseTime
  },
  data() {
    return {
      quotaNamesValidator: (rule, value, callback) => {
        if (isEmpty(value)) {
          callback(new Error(this.$t('userGroup.quotaEmptyNotice')))
          return
        }
        callback()
      },
      list: [],
      listLoading: true,
      listQuery: rawQuery,
      formTemplate: JSON.parse(JSON.stringify(constFormTemplate)),
      dialogFormVisible: false,
      dialogStatus: '',
      textMap: {
        update: this.$t('userGroup.edit'),
        create: this.$t('userGroup.add')
      },
      userList: [],
      quotaList: []
    }
  },
  created() {
    this.fetchGroupAndUsers()
    this.getQuotas()
  },
  methods: {
    fetchGroupAndUsers() {
      const query = { page: 1, limit: -1 }
      this.listLoading = true
      axios.all([
        fetchUserGroup(this.listQuery),
        fetchResearcherList(query)
      ]).then(axios.spread((groupResponse, reseacherListResponse) => {
        if (groupResponse.code === 10000) {
          this.list = groupResponse.data.items || []
        }
        if (reseacherListResponse.code === 10000) {
          this.userList = reseacherListResponse.data.items || []
        }
      })).catch((error) => {
        this.$notify({
          title: this.$t('user.retry'),
          message: this.$t('user.getDataFailed') + ',' + error,
          type: 'error',
          duration: 3000
        })
      }).finally(() => {
        this.listLoading = false
      })
    },
    fetchData() {
      this.listLoading = true
      fetchUserGroup(this.listQuery).then((response) => {
        this.list = response.data.items || []
      }).catch((error) => {
        this.$notify({
          title: this.$t('user.refresh'),
          message: this.$t('user.getUserFailed') + ',' + error,
          type: 'error',
          duration: 2000
        })
      }).finally(() => {
        this.listLoading = false
      })
    },
    refresh() {
      this.listQuery = rawQuery
      this.userList = []
      this.fetchGroupAndUsers()
      this.getQuotas()
    },
    getQuotas() {
      fetchElasticQuotaTree().then((response) => {
        this.quotaList = extractLeafNamesFromQuotaTree(response.data)
      }).catch((error) => {
        this.$notify({
          title: this.$t('user.refresh'),
          message: this.$t('user.getQuotaTreeFailed') + ',' + error,
          type: 'error',
          duration: 2000
        })
      })
    },
    resetFormTemplate(objToRefer) {
      if (!isEmpty(objToRefer)) {
        this.formTemplate = objToRefer
        return
      }
      this.formTemplate = JSON.parse(JSON.stringify(constFormTemplate))
    },
    filterQuotaNodeByGroup(quotaList) {
      var filteredQuotaList = []
      if (this.list) {
        const usingQuotaNodes = this.list.filter(x => x.spec && x.spec.quotaNames).map(x => [...x.spec.quotaNames]).flat()
        filteredQuotaList = quotaList.filter(x => usingQuotaNodes.indexOf(x) < 0)
      }
      return filteredQuotaList
    },
    handleCreate() {
      this.resetFormTemplate()
      this.dialogStatus = 'create'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    createData() {
      this.$refs['dataForm'].validate((valid) => {
        if (!valid) {
          return
        }
        createUserGroup(this.formTemplate, this.formTemplate.userNames).then((response) => {
          if (response !== null && response.code === 10000) {
            this.refresh()
            this.$notify({
              title: this.$t('user.success'),
              message: this.$t('user.createSuccess'),
              type: 'success',
              duration: 2000
            })
          } else {
            this.$notify({
              title: this.$t('user.refresh'),
              message: this.$t('user.createFailed') + ',' + response.data,
              type: 'error',
              duration: 2000
            })
          }
        }).catch((error) => {
          this.$notify({
            title: this.$t('user.refresh'),
            message: this.$t('user.createFailed') + ',' + error,
            type: 'error',
            duration: 2000
          })
        }).finally(() => {
          this.dialogFormVisible = false
          this.listLoading = false
        })
      })
    },
    getUsersByGroup(groupName, userList) {
      console.log('group ', groupName, ' userList:', userList)
      var res = userList.filter(d => d.spec && (d.spec.groups || []).includes(groupName)).map(d => d.spec.userName)
      console.log('group ' + groupName + ' users:', res)
      return res
    },
    updateData() {
      this.$refs['dataForm'].validate((valid) => {
        if (!valid) {
          this.dialogFormVisible = false
          return
        }
        console.log('group to update:', this.formTemplate, this.userList)
        const userNamesSet = this.formTemplate.userNames.filter((v, i, self) => self.indexOf(v) === i)
        const newUsers = this.userList.filter(x => x.spec && userNamesSet.indexOf(x.spec.userName) >= 0)
        updateUserGroup(this.formTemplate, newUsers).then(response => {
          if (response !== null && response.code === 10000) {
            this.refresh()
            this.$notify({
              title: this.$t('user.success'),
              message: this.$t('user.updateSuccess'),
              type: 'success',
              duration: 2000
            })
            return
          }
          this.$notify({
            title: this.$t('user.refresh'),
            message: this.$t('user.updateFailed') + ',' + response.data,
            type: 'error',
            duration: 2000
          })
        }).catch((error) => {
          this.$notify({
            title: this.$t('user.refresh'),
            message: this.$t('user.updateFailed') + ',' + error,
            type: 'error',
            duration: 2000
          })
        }).finally(() => {
          this.dialogFormVisible = false
        })
      })
    },
    deleteData() {
      this.$refs['dataForm'].validate((valid) => {
        if (!valid) {
          this.dialogFormVisible = false
          return
        }
        console.log('group to delete:', this.formTemplate)
        deleteUserGroup(this.formTemplate).then(response => {
          if (response !== null && response.code === 10000) {
            this.refresh()
            this.$notify({
              title: this.$t('user.success'),
              message: this.$t('user.deleteSuccess'),
              type: 'success',
              duration: 2000
            })
            return
          }
          this.$notify({
            title: this.$t('user.refresh'),
            message: this.$t('user.deleteFailed') + ',' + response.data,
            type: 'error',
            duration: 2000
          })
        }).catch((error) => {
          this.$notify({
            title: this.$t('user.refresh'),
            message: this.$t('user.deleteFailed') + ',' + error,
            type: 'error',
            duration: 2000
          })
        }).finally(() => {
          this.dialogFormVisible = false
        })
      })
    },
    handleUpdate(userGroup) {
      var users = this.getUsersByGroup(userGroup.metadata.name, this.userList)
      this.resetFormTemplate(fromK8sUserGroup(userGroup, users))
      this.dialogStatus = 'update'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    handleDelete(userGroup) {
      var users = this.getUsersByGroup(userGroup.metadata.name, this.userList)
      this.resetFormTemplate(fromK8sUserGroup(userGroup, users))
      console.log('group to delete:', this.formTemplate)
      this.listLoading = true
      deleteUserGroup(this.formTemplate).then(response => {
        if (response !== null && response.code === 10000) {
          this.refresh()
          this.$notify({
            title: this.$t('user.success'),
            message: this.$t('user.deleteSuccess'),
            type: 'success',
            duration: 2000
          })
          return
        }
        this.$notify({
          title: this.$t('user.refresh'),
          message: this.$t('user.deleteFailed') + ',' + response.data,
          type: 'error',
          duration: 2000
        })
      }).catch((error) => {
        this.$notify({
          title: this.$t('user.refresh'),
          message: this.$t('user.deleteFailed') + ',' + error,
          type: 'error',
          duration: 2000
        })
      }).finally(() => {
        this.listLoading = false
      })
    }
  }
}
</script>

<style lang="scss" scoped>
.el-select {
  display: block;
  //padding-right: 8px;
}
</style>
