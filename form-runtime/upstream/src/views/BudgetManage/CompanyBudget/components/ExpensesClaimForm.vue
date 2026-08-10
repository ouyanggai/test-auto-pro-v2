<!--
 * @Descripttion: 费用报销单-增查改
 * @Author: zhengzetao
 * @Date: 2022-06-15
-->

<template>
  <!-- class='outer' -->
  <!-- <div> -->
  <div class='outer'>
    <!-- <div :class='{ "outer": operaType != "check" }'> -->
    <!-- <el-dialog title="费用报销单" :visible="visible" :close-on-click-modal="false" custom-class="dialog-fullscreen" center
      @close='handleClose'> -->
    <h3 style="text-align:center;">费用报销单</h3>
    <!-- <h2 style="text-align:center;margin-bottom:20px">{{ selectFlowName }}</h2> -->
    <!-- <el-cascader v-model="testValue" :options="testOptions"></el-cascader> -->
    <div style="height:21px;line-height:21px;"><span v-if="operaType == 'add'">当前公司：{{ currentCompany }}</span></div>
    <el-form :model="infoForm" :rules="infoFormRules" ref="infoForm" label-width="137px" label-position="right"
      id="expendForm">
      <el-card shadow="never">
        <el-row>
          <el-col :span="12">
            <!-- <el-form-item label="报销单位" prop="companyId"> -->
            <el-form-item label="报销单位" prop="expenseCompanyId">
              <el-select v-model="infoForm.expenseCompanyId" placeholder="选择报销单位"
                :disabled="isDisabled('expenseCompanyId')" style="width:100%;height:30px;line-height:30px;" filterable
                @change="changeExpenseCompany" class="expenseCompany">
                <el-option v-for="item in companyList" :key="item.id" :label="item.name" :value="item.id">
                </el-option>
                <el-option v-if="infoForm.expenseCompanyId && !hasValue(infoForm.expenseCompanyId)"
                  :label="infoForm.expenseCompanyName" :value="infoForm.expenseCompanyId"></el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="单据附件数量(张)" prop="attachmentCount" style="border-right: none;">
              <el-input v-model.trim="infoForm.attachmentCount" :disabled="isDisabled('attachmentNum')"
                placeholder="请输入单据附件数量">
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="6">
            <el-form-item label="申请人" style="border-right: none;text-align: center;">
              <div style="padding:0 2px;text-align: center;width:100%;">{{userName}}</div>
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="申请日期" style="border-left: 1px solid rgb(153,153,153);">
              <div style="padding:0 2px;text-align: center;width:100%;">{{infoForm.createDate | getDate}}</div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="是否冲借（请）款" style="border-right: none;">
              <el-radio-group v-model="infoForm.repay" @change="repayChange" :disabled="isDisabled('repay')">
                <el-radio :label="null">无</el-radio>
                <el-radio label="repayLoan">冲借款</el-radio>
                <el-radio label="repayRequest">冲请款</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="24">
            <el-form-item label="项目名称" prop="projectId" style="border-bottom: none;border-right: none;">
                <el-select v-model="infoForm.projectId" placeholder="选择项目"
                :disabled="isDisabled('projectInfo')" style="width:100%;height:30px;line-height:30px;" filterable
                clearable
                @change="changeProject" >
                <el-option v-for="item in projectList" :key="item.id" :label="item.name" :value="item.id">
                </el-option>
                <el-option v-if="infoForm.projectId && !hasProjectValue(infoForm.projectId)"
                  :label="infoForm.expenseProjectName" :value="infoForm.projectId"></el-option>
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <!-- 冲借款 -->
        <div v-show="infoForm.repay">
          <div v-if="infoForm.repay == 'repayLoan'" class="sub-title">关联借款单</div>
          <div v-if="infoForm.repay == 'repayRequest'" class="sub-title">关联请款单</div>
          <div class="sub-title-button" v-if="operaType == 'add' || operaType == 'edit'"
            style="border-bottom:1px solid rgb(153,153,153);">
            <el-button type="primary" icon="el-icon-plus" @click="openRepay">
              <span v-if="infoForm.repay == 'repayLoan'">关联借款单</span>
              <span v-if="infoForm.repay == 'repayRequest'">关联请款单</span>
            </el-button>
            <!-- <span class="show-flag-list" @click="showFlagList.flag0 = !showFlagList.flag0">
              {{ showFlagList.flag0 ? '收起' : '展开' }}
              <i class="el-icon-d-arrow-left" :class="{ 'show-content': showFlagList.flag0 }"></i></span> -->
          </div>
          <div v-show="showFlagList.flag0">
            <el-table :data="infoForm.accountDetailedVoList" border :summary-method="getExpendAccountSummaries"
              align="center" show-summary style="width: 100%;border-top:none;"
              v-if="infoForm.accountDetailedVoList && infoForm.accountDetailedVoList.length" class="repay-table">
              <el-table-column :label="infoForm.repay == 'repayLoan' ? '借款流程' : '请款流程'" width="150">
                <template slot-scope="scope">
                  <el-link type="primary" @click="showDetail(scope.row)">{{ scope.row.flowName }}</el-link>
                </template>
              </el-table-column>
              <el-table-column :label="infoForm.repay == 'repayLoan' ? '借款金额' : '请款金额'">
                <template slot-scope="scope">
                  <el-form-item label-width="0px" :prop="'accountDetailedVoList.' + scope.$index + '.payMoney'"
                    style="text-align:center;">
                    <div style="width:100%">
                      {{ scope.row.payMoney }}元
                    </div>
                    <!-- <div style="display: flex;">
                      <el-input v-model="scope.row.payMoney" style="width:100%;" :readonly="true"></el-input>
                      <div style="width:40px;padding-left:10px;">
                        元
                      </div>
                    </div> -->
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column :label="infoForm.repay == 'repayLoan' ? '已还金额' : '已核销金额'">
                <template slot-scope="scope">
                  <el-form-item label-width="0px" :prop="'accountDetailedVoList.' + scope.$index + '.alreadyMoney'"
                    style="text-align:center;">
                    <div style="width:100%">{{ scope.row.alreadyMoney }}元</div>
                    <!-- <div style="display: flex;">
                      <el-input v-model="scope.row.alreadyMoney" style="width:100%;" :readonly="true"></el-input>
                      <div style="width:40px;padding-left:10px;">
                        元
                      </div>
                    </div> -->
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="冻结金额">
                <template slot-scope="scope">
                  <el-form-item label-width="0px" :prop="'accountDetailedVoList.' + scope.$index + '.freezeMoney'"
                    style="text-align:center;">
                    <div style="width:100%">{{ scope.row.freezeMoney }}元</div>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column :label="infoForm.repay == 'repayLoan' ? '未还金额' : '未核销金额'">
                <template slot-scope="scope">
                  <el-form-item label-width="0px" :prop="'accountDetailedVoList.' + scope.$index + '.notMoney'"
                    style="text-align:center;">
                    <div style="width:100%">{{ scope.row.notMoney }}元</div>
                    <!-- <el-input v-model="scope.row.notMoney" style="width:100%;" :readonly="true"></el-input>元 -->
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column prop="thisMoney" width="130px">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span v-if="infoForm.repay == 'repayLoan'">本次还款金额</span>
                  <span v-if="infoForm.repay == 'repayRequest'">本次核销金额</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label-width="0px" :prop="'accountDetailedVoList.' + scope.$index + '.thisMoney'"
                    :rules="infoFormRules.thisMoney">
                    <div style="display:flex;width:100%;">
                      <el-input-number :disabled="isDisabled('thisMoney')" v-model="scope.row.thisMoney"
                        style="width:100%;flex-grow:1;" :controls="false" :min="0.00" :precision="2" :step="0.1"
                        :max="scope.row.max"></el-input-number>
                      <div style="padding:0 5px;">
                        元
                      </div>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="50px" v-if="!isDisabled('thisMoney')" class-name="action">
                <template slot-scope="scope">
                  <i class="el-icon-delete-solid" title="删除" style="color: #1989FA;font-size: 16px;cursor: pointer;"
                    @click="removeAccountDetailed(scope.$index)"></i>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="50px" v-else class-name="action">
                <template slot-scope="scope">
                  <el-button @click="showDetail(scope.row)" type="text">详情</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>
        <!-- 费用明细 -->
        <div class="box-card">
          <div class="sub-title">费用明细</div>
          <div class="sub-title-button" v-if="operaType == 'add' || operaType == 'edit'">
            <el-button type="primary" icon="el-icon-plus" @click="addExpendDetailList">新增
            </el-button>
            <!-- <span class="show-flag-list" @click="showFlagList.flag2 = !showFlagList.flag2">
              {{ showFlagList.flag2 ? '收起' : '展开' }}
              <i class="el-icon-d-arrow-left" :class="{ 'show-content': showFlagList.flag2 }"></i></span> -->
          </div>
          <div v-show="showFlagList.flag2">
            <el-table :data="infoForm.expenseDetailList" border id="expendDetailTable"
              :summary-method="getExpendDetailSummaries" show-summary style="width: 100%">
              <el-table-column label="部门" width="160px">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>部门</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'expenseDetailList.' + scope.$index + '.depId'"
                    :rules="[{ required: true, message: '请选择', trigger: 'change' }]" v-if="!isDisabled('expendDetailType')">
                  <!-- <el-form-item label="" label-width="0px" :prop="'expenseDetailList.' + scope.$index + '.depId'" v-if="!isDisabled('expendDetailType')"> -->
                    <!-- <div style="width: 100%;cursor: pointer;border:1px solid #DCDFE6;border-radius:4px;" @click="selectDepart(scope.$index)">
                      <span v-if="scope.row.depName">{{scope.row.depName}}</span>
                      <span v-else style="color: #C0C4CC;">
                        请选择部门
                      </span>
                    </div> -->
                    <el-select v-model="scope.row.depId" filterable placeholder="选择部门" style="width:100%;height:35px;line-height:35px;"
                      @change="val=>departChange(val,scope.$index)" :disabled="isDisabled('expendDetailType')" >
                      <el-option v-for="(item, index) in companyDepartList" :key="index" :label="item.departmentName"
                        :value="item.id">
                      </el-option>
                      <el-option v-if="infoForm.expenseDetailList[scope.$index] && infoForm.expenseDetailList[scope.$index].depId &&
                                !companyDepartList.find(item=>item.id == infoForm.expenseDetailList[scope.$index].depId)"
                                :label="infoForm.expenseDetailList[scope.$index].depName" :value="infoForm.expenseDetailList[scope.$index].depId"></el-option>
                    </el-select>
                  </el-form-item>
                  <span v-else style="white-space: break-all;">
                      {{ infoForm.expenseDetailList[scope.$index].depName }}
                  </span>
                  <!-- <div v-else>{{ scope.row.depName }}</div> -->
                </template>
              </el-table-column>
              <el-table-column label="费用类型" width="150px">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>费用类型</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'expenseDetailList.' + scope.$index + '.type'"
                    :rules="infoFormRules.expenseDetailList.expendDetailType">
                    <el-select v-model="scope.row.type" filterable clearable placeholder="选择费用类型"
                      :disabled="isDisabled('expendDetailType')" style="width:100%;height:35px;line-height:35px;" @change="val=>selectType(val,scope.$index)">
                      <el-option v-for="(item, index) in oaExpendTypeList" :key="index" :label="item.name"
                        :value="item.name">
                      </el-option>
                    </el-select>
                  </el-form-item>
                </template>
              </el-table-column>
              <!-- <el-table-column label="金额" prop="money" width="130px">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>金额(元)</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'expenseDetailList.' + scope.$index + '.money'"
                    :rules="infoFormRules.expenseDetailList.expendDetailNum">
                    <div style="display: flex;width:100%;">
                      <el-input-number v-model="scope.row.money" :min="0.00" :precision="2" :step="0.1"
                        :controls="false" :disabled="isDisabled('expendDetailNum')"
                        style="flex-grow:1;width:130px;height:35px;line-height:35px;text-align:left;" v-focusSelect>
                      </el-input-number>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column> -->
              <el-table-column label="备注" >
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px">
                    <el-input type="textarea" autosize v-model.trim="scope.row.remark"
                      :disabled="isDisabled('expendDetailRemark')" placeholder="请输入备注" style="width:100%;"></el-input>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column :label="'附件' + (fileLength ? '(' + fileLength + ')' : '')" width="">
                <!-- <el-form-item label="" label-width="0px"> -->
                <template slot-scope="scope">
                  <!-- <div>{{ scope.row }}</div> -->
                  <fileUpload :row="scope.row" :uploadLimit="50" :disabled="isDisabled('expendDetailFile')"
                    :multiple="true" style="margin-left:2px;"></fileUpload>
                </template>
                <!-- </el-form-item> -->
              </el-table-column>
              <el-table-column label="是否增值税专票" width="85px">
                <template slot-scope="scope">
                  <el-select @change="val=>changeTax(val,scope.$index)" v-model="scope.row.isTax" style="height:35px;line-height:35px;" :disabled="isDisabled('expendDetailType')">
                    <el-option label="否" value="1">
                    </el-option>
                    <el-option label="是" value="2">
                    </el-option>
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="税前金额" width="90px">
                <template slot-scope="scope">
                  <el-input-number v-model="scope.row.beforeTax" :min="0.00" :precision="2" :step="0.1"
                        :controls="false" :disabled="scope.row.isTax == '1' || isDisabled('expendDetailType')" style="width:100%;flex:1;text-align:left;height:35px;line-height:35px;"
                        @input="inputDetailTax(scope.$index)" v-focusSelect >
                      </el-input-number>
                </template>
              </el-table-column>
              <el-table-column label="税额" width="80px">
                <template slot-scope="scope">
                    <div style="display: flex;">
                      <el-input-number v-model="scope.row.tax" :min="0.00" :precision="2" :step="0.1" :controls="false"
                      :disabled="scope.row.isTax == '1' || isDisabled('expendDetailType')" style="width:100%;flex:1;text-align:left;height:35px;line-height:35px;"
                        @input="inputDetailTax(scope.$index)" v-focusSelect>
                      </el-input-number>
                    </div>
                </template>
              </el-table-column>
              <!-- <el-table-column label="价税合计" >
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>价税合计(元)</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'expenseDetailList.' + scope.$index + '.money'"
                    :rules="infoFormRules.expenseDetailList.expendDetailNum">
                    <div style="display: flex;width:100%;">
                      <el-input-number v-model="scope.row.money" :min="0.00" :precision="2" :step="0.1"
                        :controls="false" :disabled="scope.row.invoiceType == '2' || isDisabled('expendDetailNum')"
                        style="flex-grow:1;width:130px;height:35px;line-height:35px;text-align:left;" v-focusSelect>
                      </el-input-number>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column> -->
              <el-table-column label="价税合计" prop="money" width="130px">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>价税合计(元)</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'expenseDetailList.' + scope.$index + '.money'"
                    :rules="infoFormRules.expenseDetailList.expendDetailNum">
                    <div style="display: flex;width:100%;">
                      <el-input-number v-model="scope.row.money" :min="0.00" :precision="2" :step="0.1"
                        :controls="false" :disabled="scope.row.isTax == '2' || isDisabled('expendDetailNum')"
                        style="flex-grow:1;width:130px;height:35px;line-height:35px;text-align:left;" v-focusSelect>
                      </el-input-number>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="50px" v-if="operaType == 'add' || operaType == 'edit'"
                class-name="action">
                <template slot-scope="scope">
                  <i class="el-icon-delete-solid" title="删除" style="color: #1989FA;font-size: 16px;cursor: pointer;"
                    @click="deleteExpendDetail(scope.$index)" v-if="infoForm.expenseDetailList.length > 1"></i>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>
        <!-- 关联固定资产 -->
        <div style="display: flex;border-bottom:1px solid rgb(153,153,153);">
          <div style="width: 365px;border-right: 1px solid rgb(153,153,153);padding:5px;">关联《固定资产购置申请表》/《低值易耗品》</div>
          <div style="flex:1;">
            <div v-for="(item ,index) in infoForm.assetsProcessVoList" :key="index" >
              <el-button @click="showAssetsDetail(item)" type="text"  style="font-size:15px;">{{item.flowName}}</el-button>
            </div>
          </div>
          <div style="width: 131px;border-left: 1px solid rgb(153,153,153);display: flex;align-items: center;justify-content: center;">
            <el-button type="primary" @click="showAssetsList" v-if="!isDisabled('isRelateAssets')">选择</el-button>
          </div>
        </div>
        <!-- 费用预算类型 -->
        <div class="box-card" v-if="infoForm.repay != 'repayRequest'" shadow="never">
          <div class="sub-title">费用预算类型</div>
          <div class="sub-title-button" v-if="operaType == 'add' || operaType == 'edit'">
            <el-button type="primary" icon="el-icon-plus" @click="addBudgetTypeList">新增</el-button>
          </div>
          <!-- <span class="show-flag-list" @click="showFlagList.flag1 = !showFlagList.flag1">
              {{ showFlagList.flag1 ? '收起' : '展开' }}
              <i class="el-icon-d-arrow-left" :class="{ 'show-content': showFlagList.flag1 }"></i></span> -->
          <div v-show="showFlagList.flag1">
            <el-table :key="expenseBudgetTableKey" ref="expenseBudgetTable" :data="infoForm.expenseBudgetList" border id="budgetTypeTable"
              :summary-method="getExpendBudgetTypeSummaries" show-summary style="width: 100%">
              <el-table-column label="序号" width="120" class-name="index">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>序号</span>
                </template>
                <template slot-scope="scope">
                  <div v-if="isTopCompany">
                    <el-form-item label="" label-width="0px"
                      :prop="'expenseBudgetList.' + scope.$index + '.companyNumber'"
                      :rules="infoFormRules.expenseBudgetList.companyNumber">
                      <el-select v-model.trim="scope.row.companyNumber"
                        @change="val => changeCompanyNumber(val, scope.$index)">
                        <el-option v-for="item in companyNumberList" :key="item.id" :value="item.number"
                          :labe="item.number"></el-option>
                      </el-select>
                    </el-form-item>
                  </div>
                  <div v-else>
                    <el-form-item label="" label-width="0px"
                      :prop="'expenseBudgetList.' + scope.$index + '.companyNumber'"
                      :rules="infoFormRules.expenseBudgetList.companyNumber">
                      <el-input v-model.trim="scope.row.companyNumber" style="width:100%;height:35px;line-height:35px"
                        :readonly="true"></el-input>
                    </el-form-item>
                  </div>

                </template>
              </el-table-column>
              <el-table-column label="费用预算类型">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>费用预算类型</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'expenseBudgetList.' + scope.$index + '.allChildId'"
                    :rules="infoFormRules.expenseBudgetList.expendType" style="position: relative;">
                    <!-- <template v-if="!isDisabled('expendType')"> -->
                    <div style="position: absolute;left: 0;top:0;width: 100%;opacity: 0;" v-if="scope.row.companyNumber">
                      <el-cascader v-model="scope.row.allChildId" clearable
                        @change="(value) => handleChange(value, scope.$index)" :props="props"
                        :options="infoForm.expenseBudgetList[scope.$index].departmentList"
                        :disabled="isDisabled('expendType')" style="width:100%;height:35px;line-height:35px;"
                        @focus="cascFocus(scope.$index)" filterable :ref="'escader'+scope.$index" >
                        <template slot-scope="{ node, data }">
                          <span>{{ data.name }}</span>
                          <span v-if="data.projectName"
                            style="color:rgb(145,145,145);font-size:12px;margin-left:5px;">项目:{{ data.projectName
                            }}</span>
                        </template>
                      </el-cascader>
                    </div>
                    <div v-else  style="position: absolute;left: 0;top:0;width: 100%;height: 100%;" @click="noCompanyNumber"></div>
                    <!-- </template> -->
                    <!-- <template v-else> -->
                    <div :class="{'escaDisable':isDisabled('expendType')}"
                      style="color:#606266;text-align: center;width:100%;border: 1px solid #E4E7ED;border-radius: 4px;height: 35px;">
                      <span v-if="scope.row.departmentName">{{ scope.row.departmentName }} / {{ scope.row.budgetName
                        }}</span>
                    </div>
                    <!-- </template> -->
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="金额" prop="money" width="200">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>金额</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'expenseBudgetList.' + scope.$index + '.money'"
                    :rules="infoFormRules.expenseBudgetList.expendTypeNum">
                    <div style="display: flex;">
                      <el-input-number v-model="scope.row.money" :min="0.00" :precision="2" :step="0.1"
                        :disabled="isDisabled('expendTypeNum')" :controls="false"
                        style="width:100%;height:35px;line-height:35px" v-focusSelect>
                      </el-input-number>
                      <div style="padding:0 5px;line-height:35px;">
                        元
                      </div>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column>
              <!-- <el-table-column label="备注" prop="remark">
              <template slot-scope="scope">
                <el-form-item label="" label-width="0px">
                  <el-input type="textarea" :rows="1" v-model.trim="scope.row.remark"
                    :readonly="isDisabled('expendDetailRemark')" placeholder="请输入备注" style="width:100%;"></el-input>
                </el-form-item>
              </template>
            </el-table-column> -->
              <el-table-column label="操作" width="50px" v-if="operaType == 'add' || operaType == 'edit'"
                class-name="action">
                <template slot-scope="scope">
                  <i class="el-icon-delete-solid" title="删除" style="color: #1989FA;font-size: 16px;cursor: pointer;"
                    @click="deleteBudgetType(scope.$index)" v-if="infoForm.expenseBudgetList.length > 1"></i>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>
        <!-- 增值税专票信息 -->
        <div class="box-card">
          <div class="sub-title">增值税专票信息</div>
          <!-- <div v-if="operaType == 'add' || operaType == 'edit'" class="sub-title-button">
            <el-button type="primary" icon="el-icon-plus" @click="addValueAddedTaxList">新增
            </el-button>
          </div> -->
          <div v-show="showFlagList.flag3">
            <el-table :data="infoForm.taxInfoList" border id="valueAddedTaxTable"
              :summary-method="getValueAddedTaxSummaries" show-summary style="width: 100%">
              <el-table-column label="" width="60px">
              </el-table-column>
              <el-table-column label="税前金额" prop="money">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>税前金额</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'taxInfoList.' + scope.$index + '.money'"
                    :rules="infoFormRules.taxInfoList.taxPreNum">
                    <div style="display: flex;">
                      <!-- <el-input-number v-model="scope.row.money" :min="0.00" :precision="2" :step="0.1"
                        :controls="false" :disabled="isDisabled('taxPreNum')" style="width:100%;flex:1;text-align:left;"
                        @input="inputTax(scope.$index)" v-focusSelect>
                      </el-input-number> -->
                      <el-input-number v-model="scope.row.money" :min="0.00" :precision="2" :step="0.1"
                        :controls="false" :disabled="true" style="width:100%;flex:1;text-align:left;"
                        @input="inputTax(scope.$index)" v-focusSelect>
                      </el-input-number>
                      <div style="padding:0 5px;">
                        元
                      </div>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="税额" prop="tax">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>税额</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'taxInfoList.' + scope.$index + '.tax'"
                    :rules="infoFormRules.taxInfoList.taxNum">
                    <div style="display: flex;">
                      <!-- <el-input-number v-model="scope.row.tax" :min="0.00" :precision="2" :step="0.1" :controls="false"
                        :disabled="isDisabled('taxNum')" style="width:100%;flex:1;text-align:left;"
                        @input="inputTax(scope.$index)" v-focusSelect>
                      </el-input-number> -->
                      <el-input-number v-model="scope.row.tax" :min="0.00" :precision="2" :step="0.1" :controls="false"
                        :disabled="true" style="width:100%;flex:1;text-align:left;"
                        @input="inputTax(scope.$index)" v-focusSelect>
                      </el-input-number>
                      <div style="width:40px;padding-left:10px;">
                        元
                      </div>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="价税合计" prop="totalAmount">
                <template slot="header">
                  <span style="color: #F56C6C;margin-right: 4px;">*</span>
                  <span>价税合计</span>
                </template>
                <template slot-scope="scope">
                  <el-form-item label="" label-width="0px" :prop="'taxInfoList.' + scope.$index + '.totalAmount'">
                    <div style="display: flex;">
                      <el-input-number v-model="scope.row.totalAmount" :min="0.00" :precision="2" :step="0.1"
                        :controls="false" :disabled="true" style="width:100%;flex:1;text-align:left;">
                      </el-input-number>
                      <div style="width:40px;padding-left:10px;">
                        元
                      </div>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column>
              <!-- <el-table-column label="操作" width="50px" v-if="operaType == 'add' || operaType == 'edit'"  class-name="action">
                <template slot-scope="scope">
                  <i class="el-icon-delete-solid" title="删除" style="color: #1989FA;font-size: 16px;cursor: pointer;"
                    @click="deleteValueAddedTax(scope.$index)"></i>
                </template>
              </el-table-column> -->
            </el-table>
          </div>
        </div>

        <!-- 入账信息 -->
        <div class="box-card">
          <div class="sub-title" style="border-bottom:1px solid rgb(153,153,153);">入账信息</div>
          <!-- <div style="margin:5px 0;">
          <span class="show-flag-list" @click="showFlagList.flag4 = !showFlagList.flag4">
            {{ showFlagList.flag4 ? '收起' : '展开' }}
            <i class="el-icon-d-arrow-left" :class="{ 'show-content': showFlagList.flag4 }"></i></span>
        </div> -->
          <div>
            <el-form-item label="" label-width="0px" class="accountinform">
              <el-table :data="filteredAccountInfoList" border id="budgetTypeTable"
                :summary-method="getAccountingInfoSummaries" show-summary style="width: 100%">
                <el-table-column label="单位或姓名">
                  <template slot-scope="scope">
                    <el-form-item>
                      <el-button type="text" v-if="scope.$index < 1 && scope.row.name != '冲借（请）款'" @click.stop="showSelect"
                        :disabled="isDisabled('accountInfoName')">选择</el-button>
                      <el-input type="textarea" style="width:100%;" autosize v-model.trim="scope.row.name"
                        :disabled="scope.row.name == '冲借（请）款' || scope.$index >= 1 || isDisabled('accountInfoName')"
                        :placeholder="scope.$index >= 1 ? '' : '请输入单位或姓名'">
                      </el-input>
                    </el-form-item>
                  </template>
                </el-table-column>
                <el-table-column label="开户行">
                  <template slot-scope="scope">
                    <el-form-item>
                      <el-input type="textarea" style="width:100%;" autosize v-model.trim="scope.row.openingBank"
                        :disabled="scope.row.name == '冲借（请）款' || scope.$index >= 1 || isDisabled('accountInfoBank')"
                        :placeholder="scope.$index >= 1 ? '' : '请输入开户行'">
                      </el-input>
                    </el-form-item>
                  </template>
                </el-table-column>
                <el-table-column label="账号">
                  <template slot-scope="scope">
                    <el-form-item>
                      <el-input type="textarea" style="width:100%;" autosize v-model.trim="scope.row.account"
                        :disabled="scope.row.name == '冲借（请）款' || scope.$index >= 1 || isDisabled('accountInfoId')"
                        :placeholder="scope.$index >= 1 ? '' : '请输入账号'">
                      </el-input>
                    </el-form-item>
                  </template>
                </el-table-column>
                <el-table-column label="金额" prop="money" width="200px">
                  <template slot-scope="scope">
                    <el-form-item>
                      <template v-if="scope.row.name != '冲借（请）款'">
                        <div style="display: flex;width:100%;">
                          <el-input-number v-model="scope.row.money" :min="0.00" :precision="2" :step="0.1"
                            :controls="false" :disabled="isDisabled('accountInfoNum')"
                            style="width:130px;height:35px;line-height:35px;text-align:left;flex-grow:1;" v-focusSelect>
                          </el-input-number>
                          <div style="padding:0 5px;line-height:35px;">
                            元
                          </div>
                        </div>
                      </template>
                      <template v-else>
                        <div style="display: flex;width:100%;">
                          <el-input-number v-model="scope.row.money" :min="0.00" :precision="2" :step="0.1"
                            :controls="false" :disabled="true"
                            style="width:130px;height:35px;line-height:35px;text-align:left;flex-grow:1;" v-focusSelect>
                          </el-input-number>
                          <div style="padding:0 5px;line-height:35px;">
                            元
                          </div>
                        </div>
                      </template>
                    </el-form-item>
                  </template>
                </el-table-column>
              </el-table>
            </el-form-item>
          </div>
        </div>
      </el-card>
    </el-form>
    <!-- infoForm.repay == 'repayLoan -->
    <el-dialog :title="infoForm.repay == 'repayLoan' ? '关联借款单' : '关联请款单'" :visible="repayVisible"
      :close-on-click-modal="false" append-to-body center @close='handleCloseRepay' width="1044px" v-if="repayVisible">
      <dy-table :fetchData="fetchData" :actions="actions" :keys="infoForm.repay == 'repayLoan' ? colKey2 : colKey"
        :list='tableData' :isPagination="true" :pagination="pagination" showCheckBox ref="dyTable"></dy-table>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleCloseRepay">取 消</el-button>
        <el-button type="primary" @click="confirmList">确 认</el-button>
      </div>
    </el-dialog>
    <!-- 固定资产流程的列表 -->
    <el-dialog :visible="assetsListVisible" :close-on-click-modal="false" append-to-body center
      @close='handleCloseAssets' width="1044px" v-if="assetsListVisible">
      <div class="search">
        <el-input style="width:120px;margin-right:5px" v-model.trim="query.flowName" @keyup.enter.native="getTaskList"
          placeholder="查找流程标题">
          <i slot="suffix" style="cursor:pointer" @click="getTaskList" class="el-input__icon el-icon-search"></i>
        </el-input>
        <el-input style="width:120px;margin-right:5px" v-model.trim="query.initiator" @keyup.enter.native="getTaskList"
          placeholder="查找发起人" >
          <i slot="suffix" style="cursor:pointer" @click="getTaskList" class="el-input__icon el-icon-search"></i>
        </el-input>
        <el-button type="primary" icon="el-icon-search" @click="getTaskList" style="border: none;">搜索</el-button>
      </div>
      <el-tabs @tab-click="tabClick" v-model="assetsTab">
        <el-tab-pane label="待办" name="backlog">
        </el-tab-pane>
        <el-tab-pane label="已办" name="finished">
        </el-tab-pane>
        <el-tab-pane label="已发" name="submitted">
        </el-tab-pane>
      </el-tabs>
      <dy-table :fetchData="fetchAssetsData" :keys="assetsKey" :list='assetsTableData' :isPagination="true"
        :pagination="assetsPagination" showCheckBox ref="dyAssetsTable"></dy-table>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleCloseAssets">取 消</el-button>
        <el-button type="primary" @click="confirmAssetsList">确 认</el-button>
      </div>
    </el-dialog>
    <!-- 表单详情 -->
    <!-- 审核弹窗(对formMakiing制作的表单的审核) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :visible.sync="examineDialogVisible" :btnVisible="false"
      :isExamine="false" :flowId="flowId" :flowNodeType="flowNodeType" :nextNodeProxyId="nextNodeProxyId"
      :formId="formId" :lastCountersignFlag="lastCountersignFlag" :nextNodeName="nextNodeName"
      :flowNodeProxyId="previewflowNodeProxyId" :jobTaskId="jobTaskId" :initiatorId="initiatorId"
      :flowInstanceId="flowInstanceId" :businessId="businessId" :companyId="companyId"
      :selectFlowType="selectFlowType" />
    <!-- 部门选择 -->
    <!-- <IndicatorHeaderDialog :visible.sync="PersonSelectDialogVisible" v-if="PersonSelectDialogVisible"
      fielSelectType="department" @selectHeader="selectHeader"/> -->
    <!-- 打印 -->
    <div id="printcontent" style="display: none;">
      <!--  -->
      <table border="0" style="table-layout: fixed">
        <tr>
          <td colspan="6">
            <h3 style="text-align:center;">费用报销单</h3>
          </td>
        </tr>
        <tr>
          <td>报销单位</td>
          <td colspan="3">{{ printObj.companyName }}</td>
          <td style="">单据附件数量(张)</td>
          <td>{{ infoForm.attachmentCount }}</td>
        </tr>
        <tr>
          <td style=" ">申请人</td>
          <td>{{ userName }}</td>
          <td style=" ">申请日期</td>
          <td>{{ infoForm.createDate.substr(0,10) }}</td>
          <td style="">是否冲借(请)款</td>
          <td>{{ infoForm.repay == 'repayLoan' ? '冲借款' : infoForm.repay == 'repayRequest' ? '冲请款' : '无' }}</td>
          <!-- <td style="">备注</td>
          <td colspan="2">{{ printObj.remark }}</td> -->
        </tr>
        <tr>
          <td style=" ">项目名称</td>
          <td colspan="5">{{infoForm.expenseProjectName}}</td>
        </tr>
        <!-- 关联借款单 -->
        <tr v-if="infoForm.repay == 'repayLoan'">
          <td colspan="6" style="">关联借款单</td>
        </tr>
        <tr v-if="infoForm.repay == 'repayLoan'">
          <td>借款流程</td>
          <td>借款金额</td>
          <td>已还金额</td>
          <td>未还金额</td>
          <td>冻结金额</td>
          <td>本次还款金额</td>
        </tr>
        <tr v-if="infoForm.repay == 'repayRequest'">
          <td>请款流程</td>
          <td>请款金额</td>
          <td>已核销金额</td>
          <td>未核销金额</td>
          <td>冻结金额</td>
          <td>本次还款金额</td>
        </tr>
        <tr v-for="(obj, index) in printObj.accountDetailedVoList" :key="index">
          <td>{{ obj.flowName }}</td>
          <td> {{ obj.payMoney }}元</td>
          <td>{{ obj.alreadyMoney }}元</td>
          <td>{{ obj.notMoney }}元</td>
          <td>{{ obj.freezeMoney }}元</td>
          <td>{{ obj.thisMoney }}元</td>
        </tr>
      </table>
      <table border="0" style="border-top:none;">
        <tr style="border-top:none;">
          <td colspan="6" style="border-top:none;">关联《固定资产购置申请表》/《低值易耗品》</td>
        </tr>
        <tr style="border-top:none;">
          <td colspan="6">
            <div v-for="(obj,index) in infoForm.assetsProcessVoList">{{obj.flowName}}</div>
          </td>
        </tr>
      </table>
      <table border="0" style="border-top:none;table-layout: fixed;">
        <tr style="border-top:none;">
          <td colspan="6" style="border-top:none;">费用明细</td>
        </tr>
        <tr>
          <td>部门</td>
          <td colspan="2">费用类型</td>
          <td>金额</td>
          <td>备注</td>
          <td>附件</td>
        </tr>
        <tr v-for="(obj, index) in infoForm.expenseDetailList" :key="obj.id">
          <td>{{ obj.depName }}</td>
          <td colspan="2">{{ obj.type }}</td>
          <td>{{ obj.money }}元</td>
          <td style="overflow:hidden;word-wrap: break-word">
            {{ obj.remark }}
          </td>
          <td>
            <!-- <div style="display:flex;flex-wrap:wrap;">
              <div v-for="val in obj.uploadFileList" :key="val.id" style="margin:0 5px 5px 0;">{{ val.name }}</div>
            </div> -->
          </td>
        </tr>
        <tr>
          <td colspan="2">合计：</td>
          <td colspan="2">
            <div style="text-align:center;border:none;">
              <span>{{ infoForm.expenseDetailList | sums }}元</span>
            </div>
          </td>
          <td colspan="2" style="border:none;">
            <div style="text-align:left;">
              <span>大写：{{ infoForm.expenseDetailList | sums | chineseMoney }}</span>
            </div>
          </td>
        </tr>
        <tr v-if="printObj.expenseBudgetList.length">
          <td colspan="6" style="">费用预算类型</td>
        </tr>
        <tr v-if="printObj.expenseBudgetList.length">
          <td style="">序号</td>
          <td colspan="4">费用预算类型</td>
          <td style="">金额</td>
        </tr>
        <tr v-for="(obj, index) in printObj.expenseBudgetList" :key="index">
          <td>{{ obj.companyNumber }}</td>
          <td colspan="4">{{ obj.departmentName +'/' + obj.budgetName }}</td>
          <td style="width:100px;">{{ obj.money }}元</td>
        </tr>
        <tr v-if="printObj.expenseBudgetList.length">
          <td colspan="5"></td>
          <td>合计：{{ printObj.expenseBudgetList | sums }}元</td>
        </tr>
        <tr v-if="infoForm.taxInfoList.length">
          <td colspan="6" style="">增值税专票信息</td>
        </tr>
        <tr v-if="infoForm.taxInfoList.length">
          <td colspan="2" style="">税前金额</td>
          <td colspan="2" style="">税额</td>
          <td colspan="2" style="">价税合计</td>
        </tr>
        <tr v-for="(obj, index) in infoForm.taxInfoList" :key="obj.id">
          <td colspan="2">{{ obj.money }}</td>
          <td colspan="2">{{ obj.tax }}</td>
          <td colspan="2">{{ obj.totalAmount }}元</td>
        </tr>
      </table>
      <table border="0" style="border-top:none;">
        <tr style="border-top:none;">
          <td colspan="6" style="border-top:none;">入账信息</td>
        </tr>
        <tr>
          <td style="width:190px;">单位或姓名</td>
          <td colspan="2" style="">开户行</td>
          <td colspan="2" style="">账号</td>
          <td style="width:100px;">金额</td>
        </tr>
        <tr v-for="(obj, index) in filteredAccountInfoList" :key="index">
          <td>
            <span>
              {{ obj.name }}
            </span>
          </td>
          <td colspan="2">
            <p><span v-if="obj.openingBank">{{ obj.openingBank }}</span></p>
          </td>
          <td colspan="2">
            <p><span v-if="obj.account">{{ obj.account }}</span></p>
          </td>
          <td>
            <p>{{ obj.money }}元</p>
          </td>
        </tr>
        <tr v-if="filteredAccountInfoList.length">
          <td colspan="5"></td>
          <td>合计：{{ infoForm.expenseInAccountInfoList | sums }}元</td>
        </tr>
      </table>
      <!-- 转发的流程和附言 -->
      <TranspondLog :printStyle="true" v-if="isTranspondFlow" :flowInstanceId="flowInstanceId" :logTableData="logTableData" :transpondFlowData="transpondFlowData" :isNoEnterprise="false"></TranspondLog>

      <!-- 流程日志 -->
      <!-- <FlowLog :flowInstanceId="$attrs.flowInstanceId" :logTableData="logTableData" :isNoEnterprise="false"></FlowLog> -->
      <div class="flow-log-container" style="margin-top: 20px;" v-if="printList.indexOf('发起人附言') > -1">
        <!-- direction="vertical" -->
        <div style="color: 000;margin-top:10px;font-size:12px;">
          <div style="background:rgb(140,140,140);border: 1px solid rgb(140,140,140);padding:8px 10px;">发起人附言</div>
          <!-- <div style="background:rgb(140,140,140);border: 1px solid rgb(140,140,140);border-bottom:none;padding:8px 10px;">附言记录</div> -->
          <div v-for="(val, index) in postscriptList" :key="index"
            style="padding:6px 10px;background:rgb(245,245,245);border:1px solid rgb(153,153,153);">
            <div style="display: flex;">
              <div style="margin-right:5px;width:80px;">{{ val.replyName || val.sendName }}</div>
              <div style="margin-right:30px;">
                {{ val.createDate }}
              </div>
            </div>
            <!-- 附言： -->
            <div style="margin-left:5px;width:100%;">{{ val.text }} </div>
            <span style="margin-left:5px;color: #47a1fb;" v-if="val.relationFileDataVos && val.relationFileDataVos.length>0"><span style="margin-right:20px" :key="file.id" v-for="file in val.relationFileDataVos">{{ file.originFileName }}</span></span>
            <div v-if="val.children.length" style="margin-left:10px;border: 1px solid #ccc;padding: 4px;margin: 5px 0px;" class="script-item-child">
              <div v-for="( childItem, childIndex) in val.children" :key="childItem.id">
                <div class="item-info-child">
                  <span style="margin-right:30px;">{{ childItem.replyName || childItem.sendName }}</span>
                  <span class="item-info-date">{{ childItem.createDate }}</span>
                </div>
                <div style="text-indent: 1rem;">{{ childItem.text }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="flow-log-container" style="margin-top: 20px;" v-if="printList.indexOf('流程日志') > -1">
        <div direction="vertical" style="color: 000;font-size:12px;">
          <div
            style="background:rgb(140,140,140);border: 1px solid rgb(140,140,140);border-bottom:none;padding:8px 10px;">
            流程日志</div>
          <div v-for="(val, index) in logTableData" :key="index"
            style="display: flex;padding:6px 10px;background:rgb(245,245,245);border:1px solid rgb(153,153,153);">
            <div style="margin-right:5px;width:80px;">{{ val.executorName }}</div>
            <div style="margin-right:5px;width:80px;">{{ val.auditStatus }} </div>
            <!-- <div style="margin-right:30px;">
              {{ val.auditStatus }}
            </div> -->
            <div style="margin-right:30px;">
              {{ val.createDate }}
            </div>
            <div>
              {{val.executeDesc}}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import fileUpload from '@/components/TableFileUpload';
import { printCss } from './oaExpendTypeList'
import { localstorageGet } from '@/utils/auth'
import { deepClone, capitalMoney } from '@/utils'
import math from '@/utils/math.js'
import DyTable from '@/components/DyTable';
import customJson from '@/components/Custom/customJson'
// import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue';
import FlowLog from '@/views/GroupApproveManage/components/flowLog.vue';
import TranspondLog from '@/views/GroupApproveManage/components/TranspondLog.vue';
import moment from "moment";
const EnterpriseExamineDialog = () => import('@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue')
// import IndicatorHeaderDialog from '@/components/IndicatorHeaderDialog.vue';
export default {
  name: 'ExpensesClaimForm',
  components: { fileUpload, DyTable, EnterpriseExamineDialog, FlowLog,TranspondLog},
  data() {
    var validateMoney = (rule, value, callback) => {
      if (Object.is(Number(value), NaN)) {
        return callback(new Error('请输入金额'));
      }

      callback();
    };
    var validateAttachmentCount = (rule, value, callback) => {
      if (value !== '' && value !== null && value !== undefined) {
        const num = Number(value);
        if (!Number.isInteger(num) || num < 0) {
          return callback(new Error('请输入大于等于0的整数'));
        }
      }
      callback();
    };
    return {
      userName:'',
      customFields: customJson,
      showFlagList: {
        flag0: true,
        flag1: true,
        flag2: true,
        flag3: true,
        flag4: true
      },
      currentCompany: localstorageGet('companyName'),
      value: [],
      oaExpendTypeList: [],//oaExpendTypeList,
      props: {
        value: 'id',
        label: 'name',
        children: 'childrenList',
      },
      infoForm: {
        expenseCompanyId: '',
        expenseCompanyName: '',
        companyId: '',
        projectId: '',
        attachmentCount: '',
        remark: '',
        repay: null,
        createDate:'',
        accountDetailedVoList: [
          // {
          // 		expenseReimbursementId:'',
          // 		payMoney:'',			  //借款金额
          //     processName:'',        //"流程名称"
          //     processId: '',      //"流程id"
          // 		alreadyMoney:'',		//已还金额
          // 		notMoney:'',			  //未还金额
          // 		thisMoney:'',			  //本次还款金额
          // 	}
        ],
        expenseBudgetList: [
          {
            companyNumber: '',
            allChildId: [],
            money: 0,
            remark: ''
          }
        ],
        expenseDetailList: [
          {
            depId:'',
            type: '',
            remark: '',
            money: '',
            attachmentIds: '',
            invoiceType: '1',
            isTax:'1', //1不是增值税 2是增值税
            uploadFileList: [
            ],
          }
        ],
        taxInfoList: [
          // {
          //   money: '',
          //   tax: '',
          //   totalAmount: '',
          //   invoiceType: 2
          // }
        ],
        expenseInAccountInfoList: [
          // {
          //   type: 1,
          //   name: '现金',
          //   openingBank: '',
          //   account: '',
          //   money: ''
          // },
          {
            type: 0,
            name: '',
            openingBank: '',
            account: '',
            money: ''
          },
          {
            type: 2,
            name: '冲借（请）款',
            openingBank: '',
            account: '',
            money: ''
          },
          // {
          //   name: '销请款',
          //   openingBank: '',
          //   account: '',
          //   money: ''
          // },

        ]
      },
      infoFormRules: {
        expenseCompanyId: [{ required: true, message: '请选择', trigger: 'change' }],
        attachmentCount: [{ required: true, trigger: 'blur', validator: validateAttachmentCount }],
        expenseBudgetList: {
          expendType: [{ required: true, message: '请选择', trigger: 'change' }],
          expendTypeNum: [{ required: true, trigger: 'blur', validator: validateMoney }],
          companyNumber: [
            { required: true, message: '请先配置归属公司序号', trigger: 'change' }
          ]
        },
        expenseDetailList: {
          depId:[{ required: true, message: '请选择部门', trigger: 'change' }],
          expendDetailType: [{ required: true, message: '请选择', trigger: 'change' }],
          expendDetailNum: [{ required: true, trigger: 'blur', validator: validateMoney }]
        },
        taxInfoList: {
          taxPreNum: [{ required: true, trigger: 'blur', validator: validateMoney }],
          taxNum: [{ required: true, trigger: 'blur', validator: validateMoney }],
          totalAmount: [{ required: true, trigger: 'blur', validator: validateMoney }]
        },
        // accountDetailedVoList:{
        thisMoney: [{ required: true, trigger: 'blur', message: '请输入金额' }]
        // }
      },
      companyList: [
      ],
      projectList: [],
      departmentList: [],
      expenseBudgetTableKey: 0,
      enableData: [],
      originFileList: [], // 存放原表单回显数据时的文件（费用明细）
      companyNumber: '',
      printObj: {
        expenseBudgetList: []
      },
      repayVisible: false,
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      colKey: {
        flowName: {
          label: '流程名称',
          minWidth: '200',
          showTooltip: true,
        },
        payCompanyName: {
          label: '付款单位',
          minWidth: '200',
          showTooltip: true,
        },
        payMoney: {
          label: '请款金额（元）',
          minWidth: '120',
        },
        alreadyMoney: {
          label: '已核销金额（元）',
          minWidth: '120',
        },
        notMoney: {
          label: '未核销金额（元）',
          minWidth: '120',
        },
        // initiator:{
        //   label:'申请人',
        //   minWidth:'100',
        // },
        // createDate:{
        //   label:'提交申请时间',
        //   minWidth:'140',
        // },
      },
      colKey2: {
        flowName: {
          label: '流程名称',
          minWidth: '200',
          showTooltip: true,
        },
        payCompanyName: {
          label: '付款单位',
          minWidth: '200',
          showTooltip: true,
        },
        payMoney: {
          label: '借款金额（元）',
          minWidth: '120',
        },
        alreadyMoney: {
          label: '已还金额（元）',
          minWidth: '120',
        },
        notMoney: {
          label: '未还金额（元）',
          minWidth: '120',
        },
        // initiator:{
        //   label:'申请人',
        //   minWidth:'100',
        // },
        // createDate:{
        //   label:'提交申请时间',
        //   minWidth:'140',
        // },
      },
      actions: [
        {
          label: '详情',
          actionFixed: 'right',
          size: 'medium',
          action: (row) => {
            this.showDetail(row)
          }
        },
      ],
      assetsKey:{
        flowName: {
          label: '流程名称',
          showTooltip: true,
          minWidth: '150',
          handle: (scope, createElement) => {
            let date = scope.row.flowInstanceName || scope.row.name
            return createElement('span', date);
          }
        },
        // auditWay: '流程名称',
        initiator: {
          label: '发起人',
          minWidth: '100'
        },
        initiatorDate: {
          label: '发起时间',
          showTooltip: true,
          minWidth: '160',
          handle: (scope, createElement) => {
            let date = scope.row.initiatorDate || scope.row.createDate
            return createElement('span', date);
          }
        },
      },
      tableData: [],
      // formVisible:false,
      // jsonData:{},
      // editData:{},
      // enableData: [],
      // disabledData: [],
      searchFlowType: '',
      flowId: '',
      flowInstanceId: '',
      jobTaskId: '',
      flowNodeType: '',
      initiatorId: '',
      nextNodeProxyId: '',
      actionType: '',
      flowType: '',
      examineDialogVisible: false,
      formId: '',
      // flowNodeProxyId:'',
      businessId: '',
      companyId: '',
      selectFlowType: '',
      previewflowNodeProxyId: '',
      printList:[],
      postscriptList:[],
      // 固定资产关联功能
      assetsListVisible:false,
      assetsTableData:[],
      assetsPagination:{
        total: 0,
        pages: 1,
        size: 10
      },
      query: {
        flowName:'',
        initiator:''
      },
      assetsTab:'backlog',
      companyDepartList:[]
      // PersonSelectDialogVisible:false
    };
  },
  filters: {
    sums(list) {
      let total = 0
      list.forEach(item => {
        total = math.add(total, item.money)
      })
      return total
    },
    chineseMoney(moeny) {
      return capitalMoney(moeny)
    },
    getDate(val){
      if(val){
        return val.substr(0,10)
      }else{
        return ''
      }
    }
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    operaType: {
      type: String,
      default: ''
    },
    id: { // 业务id
      type: String,
      default: ''
    },
    // editId: { // 编辑的业务id
    //   type: String,
    //   default: ''
    // },
    selectFlowName: {
      type: String,
      default: ''
    },
    flowProxyId: { // 流程id
      type: String,
      default: ''
    },
    flowNodeProxyId: {
      type: String,
      default: ''
    },
    logTableData:{
      type:Array,
      default:()=>{
        return []
      }
    },
    isExamine:{
      type:Boolean,
      default:false
    },
    isTranspondFlow:{
      type:Boolean,
      default:false
    },
    transpondFlowData:{
      type:Array,
      default:()=>{
        return []
      }
    },
  },
  computed: {
    isTopCompany() {
      if (localstorageGet('topCompanyId') == this.infoForm.companyId) {
        return true
      } else {
        return false
      }
    },
    fileLength() {
      let total = 0
      this.infoForm.expenseDetailList.forEach(item => {
        total += item?.uploadFileList?.length || 0
      })
      return total
    },
    isDisabled() {
      const checkKey = key => {
        // 冲请款时禁用人账信息
        if (this.infoForm.repay === 'repayRequest' && ['accountInfoName', 'accountInfoBank', 'accountInfoId', 'accountInfoNum'].includes(key)) {
          return true;
        }
        const index = this.enableData.findIndex(item => item == key);
        if (index > -1) {
          return false;
        } else {
          return true;
        }
      };
      return (key) => {
        if (this.operaType !== 'check') {
          return checkKey(key);
        } else {
          // 非流程，全部不可用
          return true;
        }
      };
    },
    filteredAccountInfoList() {
      return this.infoForm.expenseInAccountInfoList.filter(item => {
        // 冲借（请）款行：无时隐藏，冲借款和冲请款时显示
        if (item.name == '冲借（请）款') {
          return this.infoForm.repay !== null
        }
        // 普通行：冲请款时隐藏，无和冲借款时显示
        return this.infoForm.repay !== 'repayRequest'
      })
    }
  },
  watch: {
    'infoForm.accountDetailedVoList': {
      handler(accountDetailedVoList) {
        // if (accountDetailedVoList.length) {
          let total = 0
          if(accountDetailedVoList){
            accountDetailedVoList.forEach(el => {
              total = math.add(total, el.thisMoney)
            })
          }
          this.infoForm.expenseInAccountInfoList.forEach(el => {
            if (el.name == '冲借（请）款') el.money = total
          })

        // }
      },
      deep: true
    }
  },
  created() {
    let today = moment().format('YYYY-MM-DD')
    let datePoint = moment().format('YYYY-12-26')
    let currentYear = moment().format('YYYY')
    if(moment(datePoint).isBefore(today)){
      this.queryYear =  moment().add('years',1).format('YYYY')
    }else{
      this.queryYear = currentYear
    }


    this.getOaExpendTypeList()
    this.getCompanyNumber().then(async r => {

      if (r.isSuccess) {
        let companyNumberList = r.data?.dataList || []
        this.companyNumberList = companyNumberList.filter(el => {
          return el.number
        })
        let currentCompanyId = localstorageGet('companyId')
        let find = this.companyNumberList.find(item => item.companyId == currentCompanyId)
        if (find) {
          this.companyNumber = find.number
        }
      }

      this.infoForm.companyId = localstorageGet('companyId')
      // this.infoForm.expenseCompanyName =
      await this.getCompanyList();














      if (this.operaType == 'add') {








        this.infoForm.expenseBudgetList = [
          {
            companyNumber: this.companyNumber,
            allChildId: [],
            money: 0
          }
        ]
        this.infoForm.createDate = moment().format("YYYY-MM-DD")
        this.userName = localstorageGet('userName')
        this.infoForm.expenseUserName = this.userName
        this.infoForm.expenseDepName = localstorageGet('userDepartmentName')
        // this.changeTax(2,0)
        // this.infoForm.expenseProjectName = localstorageGet('userName')
        // this.getDepartmentList();
        this.getPermisionForAdd();
      }
      // 编辑和查看时才需用到
      if (this.operaType == 'check' || this.operaType == 'edit' || this.operaType == 'examine') {
        // this.getDepartmentList().then(() => {

        // });

        this.getEchDetailData().then(async () => {
          // await this.getCompanyList();
          // this.infoForm.expenseBudgetList.forEach((el, index) => {
          //   this.getDepartmentList(index); //TODO 遍历查询
          // })
          let flowInfo =await this.getInstanceId([this.id],'expense_budget') //获取申请时间
          if(flowInfo && flowInfo.length){
            if(flowInfo[0]?.createDate)this.infoForm.createDate = flowInfo[0]?.createDate
          }
          //获取项目列表
          this.getProjectList()

          // this.getProjectList();
          if (this.operaType == 'edit') {
            this.getPermisionForEdit();
          }
        });
      }
    })
  },
  mounted() {
  },
  methods: {
    inputDetailTax(index){
      if(this.infoForm.expenseDetailList[index].isTax == 2){
        let beforeTax = this.infoForm.expenseDetailList[index].beforeTax || 0
        let tax = this.infoForm.expenseDetailList[index].tax || 0
        let money = math.add(beforeTax,tax)
        this.infoForm.expenseDetailList[index].money = money
        //同步数据到增值税
        let currentSing =  this.infoForm.expenseDetailList[index].sing
        let findIndex = this.infoForm.taxInfoList.findIndex(el=>el.sing == currentSing)
        if(findIndex > -1){
          this.infoForm.taxInfoList[findIndex].money = beforeTax
          this.infoForm.taxInfoList[findIndex].tax = tax
          // this.infoForm.taxInfoList[index].totalAmount = money
        }
      }
    },
    changeTax(val,index){
      // 切换时清空当前行数据
      this.$set(this.infoForm.expenseDetailList[index], 'beforeTax', undefined)
      this.$set(this.infoForm.expenseDetailList[index], 'tax', undefined)
      this.$set(this.infoForm.expenseDetailList[index], 'money', 0)
      let sing = new Date().getTime()
      if(val == '2'){
        let obj = {
          money: '',
          tax: '',
          totalAmount: '',
          invoiceType: 2,
          sing
        }
        this.infoForm.expenseDetailList[index].sing = sing
        // this.infoForm.expenseDetailList[index].money = ''
        // this.$set(this.infoForm.taxInfoList,index,obj)
        this.infoForm.taxInfoList.push(obj)
      }else{
        this.infoForm.expenseDetailList[index].beforeTax = undefined
        this.infoForm.expenseDetailList[index].tax = undefined
        let currentSing = this.infoForm.expenseDetailList[index].sing
        let findIndex = this.infoForm.taxInfoList.findIndex(el=>el.sing == currentSing)
        if(findIndex > -1)this.infoForm.taxInfoList.splice(findIndex,1)
        this.infoForm.expenseDetailList[index].sing = ''
      }
    },
    selectType(val,index){
      let find = this.oaExpendTypeList.find(el=>el.name == val)
      if(find){
        let id = find.id
        this.infoForm.expenseDetailList[index].typeId = id
      }
    },
    getOaExpendTypeList(){

      let data = {
          data: {
              customerCode: "",
              costName:"",             //----------------------费用类型或编号查询
              enabled: "1"             //-----------------------是否启用1是0否
          },
          pagination: false,
      }
      this.$axios.post(Api.budgetManage.costTypeList,data,res=>{
        if(res.isSuccess){
          let list = res?.data?.dataList || []
          this.oaExpendTypeList = list.map(el=>{
            return {
              name:el.costName,
              id:el.id
            }
          })
        }












      })

    },
    // selectHeader(data){
    //   let depName = data?.name
    //   let depId = data?.id
    //   if(this.currentRowIndex == undefined)this.currentRowIndex = 0
    //   this.$set(this.infoForm.expenseDetailList[this.currentRowIndex],'depId',depId)
    //   this.$set(this.infoForm.expenseDetailList[this.currentRowIndex],'depName',depName)
    // },
    departChange(val,index){
      let find = this.companyDepartList.find(el=>el.id == val)
      if(find){
        // this.infoForm.expenseDetailList[index] = find.departmentName
        // console.log('this.infoForm.expenseDetailList',this.infoForm.expenseDetailList)
        this.$set(this.infoForm.expenseDetailList[index],'depName',find.departmentName)

      }
    },
    // selectDepart(index){
    //   this.currentRowIndex = index
    //   this.PersonSelectDialogVisible = true
    // },
    confirmAssetsList(){
      let assetsList = deepClone(this.$refs.dyAssetsTable.selectDatas)

      // this.infoForm.assetsProcessVoList
      if(assetsList.length){
        this.infoForm.assetsProcessVoList = assetsList.map(item=>{
          let bussinessId
          let find = item.flowInstanceBizRelevanceList.find(el=>el.otherBiz == 'assets_buy_apply')
          if(find)bussinessId = find.otherBizId
          return {
            type:'assets_buy_apply',
            processId:item.flowInstanceId || item.id,
            flowName:item.flowInstanceName || item.name,
            bussinessId
          }
        })
        this.handleCloseAssets()
      }else{
        this.$message.error('请选择至少一条流程')
      }
      // console.log('this.$refs.dyTable.selectDatas',this.$refs.dyAssetsTable.selectDatas)

      // this.infoForm.accountDetailedVoList = deepClone(this.$refs.dyTable.selectDatas)
    },
    tabClick(val){
      // console.log('val',val)
      this.fetchAssetsData()
    },
    showAssetsList(){
      this.assetsListVisible = true
    },
    getTaskList(){
      this.fetchAssetsData()
    },
    handleCloseAssets(){
      this.assetsListVisible = false
    },
    showSelect(){
      this.$fm.show('commonAccounts', { chooseData: true }).then(dialog => {
        dialog.$on('confirmed', (res) => {
          this.infoForm.expenseInAccountInfoList.forEach(el => {
            if (el.name != '冲借（请）款') {
              el.name = res.unitName
              el.openingBank = res.bank
              el.account = res.lineNumber
            }//el.money = total
          })
        });
      });
    },
    disabledRepay(key){
      if((this.infoForm.repay == 'repayRequest' || this.infoForm.repay == 'repayLoan') && !this.isDisabled('accountInfoNum')){
        return false
      }else{
        return true
      }
    },
    hasValue(id){
      let find = this.companyList.find(item=>item.id == id)
      if(find){
        return true
      }else{
        return false
      }
    },
    hasProjectValue(id){
      let find = this.projectList.find(item=>item.id == id)
      if(find){
        return true
      }else{
        return false
      }
    },
    changeExpenseCompany(val) {
      let find = this.companyList.find(item => item.id == this.infoForm.expenseCompanyId), expenseCompanyName = ''
      if (find) {
        this.infoForm.expenseCompanyName = find.name
        //获取公司相关的列表
        this.infoForm.projectId = ''
        this.infoForm.expenseProjectName = ''
        this.getProjectList()
        //通过公司获取部门列表
        let id = find.id,type = find.type
        let data = {
          id,
          flag:'enable',
          type
        }
        this.$axios.post(Api.user.findDeptVosByRefId,{data},res=>{
          if(res.isSuccess){
            this.companyDepartList = res?.data || []


            this.infoForm.expenseDetailList.forEach(el=>{
              if(el.depId){
                el.depId = ''
                el.depName = ''
              }
            })

          }
        })
      }
      // 切换报销单位时，清空已选中的冲借款/冲请款数据
      if (this.infoForm.repay && this.infoForm?.accountDetailedVoList?.length) {
        this.infoForm.accountDetailedVoList = []
      }
    },
    getCompanyId(number) {
      let find = this.companyNumberList.find(el => el.number == number)
      if (find) {
        return find.companyId
      }
    },
    changeCompanyNumber(val, index) {
      // this.infoForm.expenseBudgetList[index].companyNumber = this.getCompanyId(val)//find.companyId
      this.infoForm.expenseBudgetList[index].allChildId = ''
      this.infoForm.expenseBudgetList[index].departmentName = ''
      this.infoForm.expenseBudgetList[index].budgetName = ''
      this.getDepartmentList(index)
    },
    // fetchLogData() {
    //   return new Promise((resolve, reject) => {
    //     this.$axios.post(
    //       Api.approveManage.findRecord,
    //       {
    //         data: {
    //           flowInstanceId: this.$attrs.flowInstanceId
    //         }
    //       },
    //       res => {
    //         if (res.isSuccess) {
    //           this.logTableData = this.filterWithdraw(res.data);
    //           this.logTableData.forEach(item => {
    //             this.translateStatus(item);
    //           });
    //           resolve()
    //         } else {
    //           this.$message.error(res.message);
    //         }
    //       }
    //     );
    //   })
    // },
    // 获取表单字段值
    getFormData(id) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.getFormData,
          {
            data: {
              id//: this.flowInstanceId
            },
            // excludeFieldNames: ['auto_audit_info_']
          },
          (res) => {
            if (res.isSuccess) {
              resolve(res.data.data);
            }
          }
        );
      });
    },
    async getFormDetail(formProxyId, id) {
      const editData = await this.getFormData(id);
      for (const item in editData) {
        this.$set(this.editData, item, editData[item]);
      }
      this.$axios.post(
        Api.qualityManage.getTaskFormDetail,
        {
          data: {
            id: formProxyId
          }
        },
        (res) => {
          if (res.isSuccess) {
            let copyTemplateData = JSON.parse(res.data.templateData)
            this.setRequireByPermission(copyTemplateData.list);
            this.jsonData = deepClone(copyTemplateData);

            const fieldsTemplateList = res.data.fieldsTemplateList;
            const disabledData = fieldsTemplateList.map(item => {
              return item.englishName;
            });
            this.disabledData = disabledData;
            this.$nextTick(() => {
              this.$refs.generateForm.refresh();
              this.$refs.generateForm.disabled(disabledData, true);
            });
          }
        }
      );
    },
    // 递归表单，根据配置的表单权限，修改表单是否校验配置（如果某个表单字段本身设置必填，但是没有配置权限，那么要改为非必填；如果本身未设置必填，就算有权限，也不需要必填）
    setRequireByPermission(genList) {
      genList.map((item, key) => {
        if (item.type == 'grid') {
          item.columns.map(val => {
            this.setRequireByPermission(val.list);
          });
        } else if (item.type == 'report') {
          item.rows.map(val => {
            val.columns.map(i => {
              this.setRequireByPermission(i.list);
            });
          });
        } else if (item.type == 'inline') {
          this.setRequireByPermission(item.list);
        } else {
          if (item.model) {
            if (!this.enableData.includes(item.model)) {
              // 下面两行代码足够覆盖所有场景，即用下面两行；不够用可把注释打开，根据不同场景进行配置。
              this.$set(item.options, 'required', false);
              this.$set(item, 'rules', []);
            }
          }
        }
      });
    },
    handleCloseForm() {
      this.formVisible = false
    },
    async showDetail(row) {
      let id = row.processId
      if (this.repayList?.length) {
        let findFlow = this.repayList.find(item => item.id == id)
        if (findFlow) {
          this.previewDialog(findFlow)
        }
      } else {
        let flow = await this.getFlowById(id)
        if (flow && flow.id) {
          this.previewDialog(flow)
        }
      }
    },
    async showAssetsDetail(row){
      let id = row.processId
      let flow = await this.getFlowById(id)
      if (flow && flow.id) {
        this.previewDialog(flow)
      }
    },
    previewDialog(row) {
      this.isExamine = true;
      this.lastCountersignFlag = row.lastCountersignFlag;// 判断是否为当前节点最后一个审批人--选择下一个分支节点
      this.initiatorId = row.initiatorId;
      this.btnVisible = true;
      this.flowId = row.flowProxyId;
      this.flowInstanceId = row.id;
      this.formId = row.formProxyId;
      this.previewflowNodeProxyId = row.flowNodeProxyId;
      this.flowNodeType = row.flowNextNodeAuditType;
      this.nextNodeProxyId = row.nextNodeProxyId;
      this.nextNodeName = row.nextNodeName;
      this.jobTaskId = row.jobTaskId;
      const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
      this.businessId = find?.otherBizId || '';
      const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
      this.companyId = company?.otherBizId || '';
      this.examineDialogVisible = true;
    },
    repayChange() {
      this.infoForm.accountDetailedVoList = []
    },
    removeAccountDetailed(index) {
      this.$confirm('确认删除', '提示').then(() => {
        this.infoForm.accountDetailedVoList.splice(index, 1)
      }).catch(() => { })
    },
    confirmList() {
      this.infoForm.accountDetailedVoList = deepClone(this.$refs.dyTable.selectDatas)
      this.handleCloseRepay()
      // console.log('this.$refs.dyTable.selectDatas',this.$refs.dyTable.selectDatas)
    },
    openRepay() {
      if (!this.infoForm.expenseCompanyId) {
        return this.$message.error('请选择报销公司')
      }
      this.repayVisible = true
      //获取列表
      // request_funds 请款单
      // expense_loan 借款单
    },
    //搜索固定资产列表
    fetchAssetsData(){
      // console.log('this',this.assetsTab)
      // return
      var data = {},apiUrl = Api.qualityManage.findList
      if(this.assetsTab == 'backlog'){
          data = {
          typeId: '',
          // auditWay:'assets_buy_apply',
          taskStatus: 'pending',
          auditWayList: ['assets_buy_apply','low_value_things'],//this.sFlowTypeList,
          useScope: 'invest',
          flowInstanceBizRelevance: {
            planId: null,
            stationId: null,
            resourcesId: null
          },
          flowInstanceBizRelevanceList: [
            {
              otherBiz: 'company',
              otherBizId: ''
            }
          ],
          ...this.query
        };
      }else if(this.assetsTab == 'finished'){
        data = {
          executorId: this.$store.state.user.userId,
          // auditWay:'assets_buy_apply',
          auditWayList: ['assets_buy_apply','low_value_things'],
          useScope: 'invest',
          typeId: '',
          taskStatus: 'done',
          flowInstanceBizRelevance: {
            planId: null,
            stationId: null,
            resourcesId: null
          },
          flowInstanceBizRelevanceList:[
            {
              otherBiz:'company',
              otherBizId:''
            }
          ],
          ...this.query
        }
      }else if(this.assetsTab == 'submitted'){
        apiUrl = Api.schedule.getFlowInstanceList,
        data = {
          useScope: 'invest',
          // auditWay:'assets_buy_apply',
          auditWayList: ['assets_buy_apply','low_value_things'],
          statusList:['await_sent','run','withdraw','termination','abandon','rejected','end'],
          flowInstanceBizRelevanceList: [
            {
              otherBiz: 'company',
              otherBizId:''
            }
          ],
          ...this.query
        }
      }



      this.$axios.post(
        apiUrl,
        {
          data,
          pagination: true,
          pages: this.assetsPagination.pages,
          size: this.assetsPagination.size
        },
        res => {
          if (res.isSuccess) {
            this.assetsTableData = res.data ? res.data : [];
            this.assetsPagination.total = res.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    fetchData() {
      let type = this.infoForm.repay == 'repayLoan' ? '3' : '2'
      let data = {
        data: {
          companyId: this.infoForm.expenseCompanyId,//"8fe922a5da21445a8a26aba74d0af5e1",   报销公司ide
          status: "end",       //流程状态是否完结
          userId: this.infoForm.createrId || localstorageGet('userId'), //"1915188dd40f47e1a19cfa6b8a7ac563",   用户id
          type//:     请借款类型2请款3借款
        },
        pagination: true,
        pages: this.pagination.pages,
        size: this.pagination.size
      }
      this.$axios.post(Api.budgetManage.loanMoney, data, async res => {
        if (res.isSuccess) {
          let list = res.data || []
          let ids = [], tableData = []
          if (list.length) {
            list.forEach(item => {
              ids.push(item.id)
            })
            let auditWay = this.infoForm.repay == 'repayLoan' ? 'expense_loan' : 'request_funds'
            let flowList = await this.getInstanceId(ids, auditWay,'end')
            this.repayList = deepClone(flowList)
            tableData = list.map(el => {
              let id = el.id
              let find = flowList.find(item => {
                return item.flowInstanceBizRelevanceList.find(el => el.otherBizId == id)
              })
              let processId = find.id
              let alreadyMoney = math.subtract(el.amountRecordVo['payMoney'], el.amountRecordVo['notMoney'])// - item['freezeMoney']
              let obj = {
                id,
                payCompanyName: el.applicationFundsVo.payCompanyName,
                processId,
                flowName: find.name,
                initiator: find.initiator,
                payMoney: el.amountRecordVo.payMoney,//item.formCellData['applicationFundsVo_payMoney'],
                createDate: el.createDate,
                notMoney: el.amountRecordVo.notMoney,
                freezeMoney: el.amountRecordVo.freezeMoney,
                alreadyMoney,
                expenseReimbursementId: el.applicationFundsVo.expenseReimbursementId,
                max: math.subtract(el.amountRecordVo.notMoney, el.amountRecordVo.freezeMoney)
              }
              return obj
            })
          }
          this.pagination.total = res?.total || 0
          this.tableData = tableData
        } else {
          this.$message.error(res.message)
        }
      })
    },
    getFlowById(id) {
      const data = {
        id,
        useScope: 'invest',
        taskStatus: 'end',
        initiator: 'all',
      };
      let api = Api.schedule.getFlowInstanceList
      return new Promise((resolve, reject) => {
        this.$axios.post(api, { data, pagination: false }).then(res => {
          if (res.isSuccess) {
            let data = res?.data || []
            if (data.length) {
              resolve(data[0])
            } else {
              resolve([])
            }
          }
        });
      });
    },
    getInstanceId(ids, type,taskStatus) {
      let otherBiz = type
      const flowInstanceBizRelevanceList = [{
        otherBiz,            //流程类型
        otherBizIdList: ids, //业务id array
      }];
      const data = {
        useScope: 'invest',
        // taskStatus,//: 'end',
        initiator: 'all',
        flowInstanceBizRelevanceList
      };
      if(taskStatus){
        data.taskStatus = taskStatus
      }
      let api = Api.schedule.getFlowInstanceList
      return new Promise((resolve, reject) => {
        this.$axios.post(api, { data, pagination: false }).then(res => {
          if (res.isSuccess) {
            let data = res?.data || []
            if (data.length) {
              resolve(data)
            } else {
              resolve([])
            }
          }
        });
      });
    },
    getDetailMoney(ids) {
      let data = {
        data: {},
        ids
      }
      return new Promise((resolve, reject) => {
        this.$axios.post(Api.budgetManage.detailedMoney, data, res => {
          if (res.isSuccess) {
            resolve(res.data || [])
          } else {
            resolve([])
          }
        })
      })

    },
    handleCloseRepay() {
      this.repayVisible = false
    },
    async printPage(list) {
      this.printList = list
      // console.log('data',data)
      // return
      if(this.printList.indexOf('发起人附言')>-1 )await this.getPostScriptList()
      // console.log('flowProxyId',this.flowProxyId)
      // return
      let find = this.companyList.find(item => item.id == this.infoForm.expenseCompanyId)
      let printObj = {}
      if(find){
        printObj = {
          companyName:find.name,
        }
      }else{
        printObj = {
          companyName:this.infoForm.expenseCompanyName,
        }
      }

      let expenseBudgetList = deepClone(this.infoForm.expenseBudgetList)
      // expenseBudgetList.forEach((item, index) => {
      //   let allChildId = item.allChildId
      //   let find = this.infoForm.expenseBudgetList[index].departmentList.find(item => item.id == allChildId[0])
      //   if (find) {
      //     let childrenList = find.childrenList
      //     let findChild = childrenList.find(item => item.id == allChildId[1])
      //     if (findChild) {
      //       item.budgetType = find.departmentName || find.name + '/' + findChild.name
      //     } else {
      //       item.budgetType = find.departmentName || find.name
      //     }
      //   } else {
      //     item.budgetType = ''
      //   }
      // })
      printObj.expenseBudgetList = expenseBudgetList
      printObj.accountDetailedVoList = deepClone(this.infoForm.accountDetailedVoList)
      this.printObj = printObj
      // console.log('printObj',printObj)
      // return
      this.$nextTick(() => {

        var iframe = document.querySelector("#print-iframe");
        if (iframe) document.body.removeChild(iframe)
        // console.log('iframe',iframe)
        // if(!iframe){
        var el = document.querySelector("#printcontent");
        iframe = document.createElement('IFRAME');
        var doc = null;
        iframe.setAttribute("id", "print-iframe");
        iframe.setAttribute('style', 'position:absolute;width:0px;height:0px;left:-500px;top:-500px;');
        document.body.appendChild(iframe);
        doc = iframe.contentWindow.document;
        //这里可以自定义样式

        doc.write(`${printCss}<div id='printcontent'>${el.innerHTML}</div>`);
        doc.close();
        iframe.contentWindow.focus();
        // }
        iframe.contentWindow.print();
        if (navigator.userAgent.indexOf("MSIE") > 0) {
          document.body.removeChild(iframe);
        }
      })
    },
    hasChecked(val) {
    },
    getCompanyNumber() {
      return this.$axios.post(Api.budgetManage.getCompanyNumberList, {})
    },
    inputTax(index) {
      let tax = this.infoForm.taxInfoList[index].tax || 0, money = this.infoForm.taxInfoList[index].money || 0
      this.infoForm.taxInfoList[index].totalAmount = math.add(Number(tax), Number(money))
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    // 审批时获取字段权限的接口
    getPermisionForEdit() {
      this.$axios.post(
        Api.qualityManage.findApprovePermission,
        {
          data: {
          },
          nodeProxyId: this.flowNodeProxyId
        },
        (res) => {
          if (res.isSuccess) {
            let enableList = [];
            if (res.data && res.data.flowNodeFieldPowerTemplateList) {
              const tmpList = res.data.flowNodeFieldPowerTemplateList || [];
              enableList = tmpList.map(item => {
                return item.formFieldTemplateEnglishName;
              });
            }
            let mainRule = deepClone(this.infoFormRules)
            for (let key in mainRule) {
              if (key == 'expenseBudgetList' || key == 'expenseDetailList' || key == 'taxInfoList') {
                for (let childKey in mainRule[key]) {
                  if (enableList.indexOf(childKey) == -1) {
                    delete this.infoFormRules[key][childKey]
                  }
                }
              } else if (key == 'attachmentCount') {
                if (enableList.indexOf('attachmentNum') == -1) {
                  delete this.infoFormRules[key]
                }
              } else {
                if (enableList.indexOf(key) == -1) {
                  delete this.infoFormRules[key]
                }
              }
            }
            this.enableData = enableList;
          }
        }
      );
    },
    // 发起时获取字段权限的接口
    getPermisionForAdd() {
      this.$axios.post(
        Api.schedule.flowTemplateFindById,
        {
          data: {
            id: this.flowProxyId // 流程id
          }
        },
        (res) => {
          let enableList = [];
          if (res.data && res.data.flowNodeTemplate && res.data.flowNodeTemplate.flowNodeFieldPowerTemplateList) {
            const tmpList = res.data.flowNodeTemplate.flowNodeFieldPowerTemplateList || [];
            enableList = tmpList.map(item => {
              return item.formFieldTemplateEnglishName;
            });
          }
          let mainRule = deepClone(this.infoFormRules)
          for (let key in mainRule) {
            if (key == 'expenseBudgetList' || key == 'expenseDetailList' || key == 'taxInfoList') {
              for (let childKey in mainRule[key]) {
                if (enableList.indexOf(childKey) == -1) {
                  delete this.infoFormRules[key][childKey]
                }
              }
            } else if (key == 'attachmentCount') {
              if (enableList.indexOf('attachmentNum') == -1) {
                delete this.infoFormRules[key]
              }
            } else {
              if (enableList.indexOf(key) == -1) {
                delete this.infoFormRules[key]
              }
            }
          }
          this.enableData = enableList;
        }
      );
    },
    changeProject(val) {
      let find = this.projectList.find(el=>el.id == val)
      if(find) {
        this.infoForm.expenseProjectName = find.name
      } else {
        this.infoForm.expenseProjectName = ''
      }
      //     allChildId: [],
      //     money: 0
      //   }
      // ];
      // this.departmentList = [];
      // this.getDepartmentList();
    },
    // 开始下载前使用业务id去获取对应的文件数组
    getAttachmentList(id) {
      return new Promise((resolve, reject) => {
        let arr = null;
        this.$axios.post(
          Api.schedule.getAttachmentList,
          {
            data: {
              relationId: id
            },
            // fileType: 'ordinaryFile'
          },
          res => {
            if (res.isSuccess) {
              if (res.data.length) {
                arr = res.data.map(x => {
                  return {
                    absolutelyFileUrl: x.fileUrl,
                    name: x.originFileName,
                    id: x.fileId, // 这个接口的fileId才是文件id
                    // fileId: x.fileId,
                    percentage: 100,
                    status: 'uploaded'
                  };
                });
              } else {
                arr = [];
                // arr = null;
              }
              resolve(arr);
            } else {
              // this.$message.error(res.message);
            }
          }
        );
      });
    },
    // 获取表单详情数据
    async getEchDetailData() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.budgetManage.getEchDetailData,
          {
            data: {
              id: this.id
            }
          },
          async res => {
            if (res.isSuccess) {
              this.originFileList = [];
              this.companyNumber = res.data?.companyNumber || this.companyNumber
              let expenseBudgetList = res?.data?.expenseBudgetList || []
              expenseBudgetList.forEach(x => {
                x.allChildId = x.allChildId.split(',');
                if (x.companyNumber === null || x.companyNumber === undefined) x.companyNumber = this.companyNumber
              });
              res.data.expenseDetailList.forEach(el=>{
                let relationFileDatas = el?.relationFileDatas || []
                let list = relationFileDatas.map(x=>{
                    return {
                      absolutelyFileUrl: x.fileUrl,
                      name: x.originFileName,
                      id: x.fileId, // 这个接口的fileId才是文件id
                      percentage: 100,
                      status: 'uploaded'
                    };
                })
                el.isTax = '1' //非增值税
                el.tax = undefined
                if(el.sing){
                  el.isTax = '2' //增值税
                  let sing = el.sing
                  let find = res?.data?.taxInfoList.find(el=>el.sing == sing)
                  if(find){
                    el.beforeTax = find.money
                    el.tax = find.tax
                    el.money = find.totalAmount
                  }
                }
                this.$set(el, 'uploadFileList', list);
              })
              // for (let i = 0; i < res.data.expenseDetailList.length; i++) {
              //   const x = res.data.expenseDetailList[i];
              //   const fileArr = await this.getAttachmentList(x.id);
              //   if (fileArr) {
              //     this.$set(x, 'uploadFileList', fileArr);
              //   } else {
              //     this.$set(x, 'uploadFileList', []);
              //   }
              //   this.originFileList.push(JSON.parse(JSON.stringify(fileArr)));
              // }
              expenseBudgetList.sort((a, b) => a.sort - b.sort)
              res.data.expenseDetailList.sort((a, b) => a.sort - b.sort)
              res.data.expenseInAccountInfoList.sort((a, b) => a.sort - b.sort)
              Object.assign(this.infoForm, res.data);
              this.queryYear = ''
              if(this.infoForm.expenseBudgetList.length){
                let id = this.infoForm.expenseBudgetList[0]?.budgetId
                this.queryYear = await this.getCostTypeById(id)
              }
              this.infoForm.accountDetailedVoList = res?.data?.accountDetailedVoList || []
              this.infoForm.accountDetailedVoList.forEach(item => {
                item.freezeMoney = item.amountRecordVo.freezeMoney
              })
              this.infoForm.accountDetailedVoList.sort((a, b) => a.sort - b.sort)
              this.infoForm.taxInfoList.sort((a, b) => a.sort - b.sort)
              this.infoForm.repay = null
              if (this.infoForm.accountDetailedVoList?.length) {
                this.infoForm.repay = res.data.state == 1 ? 'repayLoan' : 'repayRequest'
              }
              this.infoForm.expenseCompanyId = this.infoForm.expenseCompanyId ? this.infoForm.expenseCompanyId : this.infoForm.companyId
              this.findUserDetail(res.data.createrId)
              if(this.operaType == 'add'){
                this.changeExpenseCompany()
              }else{
                let find = this.companyList.find(item => item.id == this.infoForm.expenseCompanyId)
                if (find) {
                  //通过公司获取部门列表
                  let id = find.id,type = find.type
                  let data = {
                    id,
                    flag:'enable',
                    type
                  }
                  this.$axios.post(Api.user.findDeptVosByRefId,{data},res=>{
                    if(res.isSuccess){
                      this.companyDepartList = res?.data || []
                    }
                  })
                }
              }

              resolve();
              // console.log('this.infoForm', this.infoForm);
            } else {
              this.$message.error(res.message);
            }
          }
        );
      });
    },
    getCostTypeById(id){
      return new Promise((resolve,reject)=>{
        this.$axios.post(Api.budgetManage.getCostTypeById,{data:{id}}).then(res=>{
          if(res.isSuccess){
            resolve(res.data.annually)
          }else{
            resolve('')
          }
        })
      })

    },
    //获取人员
    findUserDetail(id){
      let data = {
        data:{
          id
        }
      }
      this.$axios.post(Api.frameworkInfo.findUserDetail,data,res=>{
        if(res.isSuccess){
          this.infoForm.userName = this.userName  = res.data.name
        }
      })
    },
    // 获取公司列表
    getCompanyList() {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.user.getSingleCompany,
          {
            data: {
              //id: this.$store.state.user.companyId
            },
            userId:this.infoForm.createrId || localstorageGet('userId')
          },
          async res => {
            if (res.isSuccess) {
              this.companyList = res.data || []
              // let companyList = data.map(item=>{

              // })
              // this.companyList = companyList.concat(projectCompanyList)
              // if (this.operaType == 'add') { // 新增的时候公司默认选择主岗公司
              //   this.infoForm.companyId = res.data.find(x => x.flag == 'mainDutyCompany').id;
              //   // this.selectCompany();
              // }
              resolve()
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    //
    getProjectCompany() {
      return new Promise((resolve, reject) => {
        let data = {
          data: {
            companyId: "",
            name: "",
          },
          pagination: false,
        }
        this.$axios.post(Api.budgetManage.projectCompany, data, res => {
          let dataList = res?.data?.dataList || []
          resolve(dataList)
          // console.log('res',res)
        })
      })
    },
    // 获取公司下的项目列表 TODO
    async getProjectList() {
      await this.$axios.post(
        Api.schedule.queryProjectForPay,
        {
          data: {
            companyVo:{
              id:this.infoForm.expenseCompanyId
            }
          }
        },
        res => {
          if (res.isSuccess) {
            this.projectList = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getBudgetTypeOfGroup(companyId) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.budgetManage.getBudgetCentralizedOfGroup,
          {},
          res => {
            if (res.isSuccess) {
              const data = res.data || [];
              const find = data.find(item => item.companyVo.id == companyId);
              if (find) {
                this.centralizedApiVos = find.centralizedApiVos[0];
                this.projectBudgetCentralizedApiVos = find.projectBudgetCentralizedApiVos
                let departmentList = this.generateDepartOption();
                // console.log('departmentList',departmentList)
                resolve(departmentList)
              }
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    generateDepartOption() {
      const { deptBudgetCentralizedVoList } = this.centralizedApiVos;
      const departOptions = []
      deptBudgetCentralizedVoList.forEach(item => {
        const { sysDepartmentVo } = item;
        departOptions.push({
          id: sysDepartmentVo.id,
          name: sysDepartmentVo.departmentName == '公司领导' ? '公司固定费用' : sysDepartmentVo.departmentName,
          hasSelect: false
        })
      });
      this.projectBudgetCentralizedApiVos.forEach(item => {
        departOptions.push({
          id: item.projectVo.id,
          name: item.projectVo.shortName || item.projectVo.name,
          hasSelect: false,
          isProject: true
        })
      })
      return departOptions
    },
    noCompanyNumber(){
      this.$message.error('未配置费用预算归属公司序号，请联系综合部人员')
    },
    cascFocus(index){
      // if(!this.infoForm.expenseBudgetList[index]?.departmentList?.length){
        this.getDepartmentList(index).then(list=>{
          this.$set(this.infoForm.expenseBudgetList[index], 'departmentList', list)
          setTimeout(()=>{
            let key = 'escader'+index
            this.$refs[key].toggleDropDownVisible(true)
          },100)
        })

      // }
    },
    // 获取公司下的部门列表项目列表和归口
    async getDepartmentList(index) {
      if (index === undefined) index = 0
      let companyId = this.getCompanyId(this.infoForm.expenseBudgetList[index].companyNumber)
      return new Promise((resolve, reject) => {
        let data = {
          data: {
            annually: this.queryYear,//new Date().getFullYear(),
            companyId,//: infoForm.expenseBudgetList//this.infoForm.chooseCompanyId,
            stringList: [1, 2],
            listString: [1, 3]
          }
        }
        this.$axios.post(Api.budgetManage.getBudgetList, data, res => {
          let list = res?.data?.dataList || []
          list.sort((a, b) => a.sort - b.sort)
          let departmentList = []
          list.forEach(item => {
            let departmentId = item.departmentId
            if (!departmentId || item.type == 3) departmentId = item.projectId
            let index = departmentList.findIndex(it => it.id == departmentId)
            let name = `${item.name}${this.transformName(item.type)}`
            let child = {
              id: item.id,
              name
            }
            if (index == -1) {
              departmentList.push({
                id: departmentId,
                name: item.departmentName,
                childrenList: [child]
              })
            } else {
              departmentList[index].childrenList.push(child)
            }
          })
          resolve(departmentList)
        })
      })
    },
    // 递归操作树
    getTreeData(data) {
      for (let i = 0; i < data.length; i++) {
        if (data[i].childrenList.length < 1) {
          data[i].childrenList = undefined;
        } else {
          this.getTreeData(data[i].childrenList);
        }
      }
      return data;
    },
    transformName(type) {
      return {
        1: '(公司归口)',
        2: '(月度归口)',
        3: '(项目归口)',
      }[type]
    },
    // 费用预算类型表格操作
    addBudgetTypeList() {
      this.infoForm.expenseBudgetList.push({
        companyNumber: this.companyNumber,
        allChildId: [],
        money: 0
      });
      let index = this.infoForm.expenseBudgetList.length - 1
      this.getDepartmentList(index)
      this.refreshExpenseBudgetTable()
    },
    deleteBudgetType(index) {
      this.infoForm.expenseBudgetList.splice(index, 1);
      this.refreshExpenseBudgetTable()
    },
    refreshExpenseBudgetTable() {
      this.expenseBudgetTableKey += 1
      this.$nextTick(() => {
        this.$refs.expenseBudgetTable?.doLayout()
      })
    },
    getExpendAccountSummaries(param) {
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        const values = data.map(item => Number(item[column.property]));
        if (column.property == 'thisMoney') {
          sums[index] = values.reduce((prev, curr) => {
            const value = Number(curr);
            if (!isNaN(value)) {
              return math.add(prev, curr);
            } else {
              return prev;
            }
          }, 0);
          sums[index] = '合计：' + sums[index].toFixed(2) + ' 元';
        }
        // if (!values.every(value => isNaN(value)) && column.property == 'thisMoney') {
        //   sums[index] = values.reduce((prev, curr) => {
        //     const value = Number(curr);
        //     if (!isNaN(value)) {
        //       return prev + curr;
        //     } else {
        //       return prev;
        //     }
        //   }, 0);
        //   sums[index] = '合计：' + sums[index].toFixed(2) + ' 元';
        // } else {
        //   sums[index] = '';
        // }
      });
      return sums;
    },
    getExpendBudgetTypeSummaries(param) { // 费用预算类型表格合计
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        const values = data.map(item => Number(item[column.property]));
        if (!values.every(value => isNaN(value)) && column.property == 'money') {
          sums[index] = values.reduce((prev, curr) => {
            const value = Number(curr);
            if (!isNaN(value)) {
              return math.add(prev, value);
            } else {
              return prev;
            }
          }, 0);
          sums[index] = '合计：' + sums[index].toFixed(2) + ' 元';
        } else {
          sums[index] = '';
        }
      });
      return sums;
    },

    // 费用明细表格操作
    addExpendDetailList() {
      this.infoForm.expenseDetailList.push({
        type: '',
        remark: '',
        attachmentIds: '',
        money: '',
        invoiceType: '1', //都是1
        isTax:'1', //默认非增值税
        uploadFileList: []
      });
    },
    deleteExpendDetail(index) {
      let currentSing = this.infoForm.expenseDetailList[index].sing
      if(currentSing){
        let findIndex = this.infoForm.taxInfoList.findIndex(el=>el.sing == currentSing)
        if(findIndex > -1)this.infoForm.taxInfoList.splice(findIndex,1)
      }
      this.infoForm.expenseDetailList.splice(index, 1);
    },

    getExpendDetailSummaries(param) { // 费用明细表格合计
      // console.log(param);
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        const values = data.map(item => Number(item[column.property]));
        if (!values.every(value => isNaN(value))) {
          sums[index] = values.reduce((prev, curr) => {
            const value = Number(curr);
            if (!isNaN(value)) {
              return prev + curr;
            } else {
              return prev;
            }
          }, 0);
          this.expendSum = sums[index].toFixed(2)
          // sums[index] = '合计：' + this.expendSum  + ' 元 \r\n 大写'
          sums[index] = `合计：${this.expendSum}元
                        大写：${capitalMoney(this.expendSum)}`
          // sums[index+1] = '大写：' //+ capitalMoney(sums[index].toFixed(2));
        } else {
          // if(index == 8){
          //   // let total = sums[index-1].toFixed(2)
          //   sums[index] = `大写：${capitalMoney(this.expendSum)}`;
          // }else{
            sums[index] = '';
          // }
        }
      });
      return sums;
    },

    // 增值税专票信息表格操作
    addValueAddedTaxList() {
      this.infoForm.taxInfoList.push({
        money: '',
        tax: '',
        totalAmount: '',
        invoiceType: 2
      });
    },
    deleteValueAddedTax(index) {
      this.infoForm.taxInfoList.splice(index, 1);
    },
    getValueAddedTaxSummaries(param) { // 增值税专票信息表格合计
      // console.log(param);
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        if (index === 0) {
          sums[index] = '合计：';
          return;
        }
        const values = data.map(item => Number(item[column.property]));
        if (!values.every(value => isNaN(value))) {
          sums[index] = values.reduce((prev, curr) => {
            const value = Number(curr);
            if (!isNaN(value)) {
              return prev + curr;
            } else {
              return prev;
            }
          }, 0);
          sums[index] = sums[index].toFixed(2) + ' 元';
        } else {
          sums[index] = '';
        }
      });
      return sums;
    },

    // 入账信息表格操作
    getAccountingInfoSummaries(param) { // 入账信息表格合计
      // console.log(param);
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        const values = data.map(item => Number(item[column.property]));
        if (!values.every(value => isNaN(value))) {
          sums[index] = values.reduce((prev, curr) => {
            const value = Number(curr);
            if (!isNaN(value)) {
              return prev + curr;
            } else {
              return prev;
            }
          }, 0);
          sums[index] = '合计：' + sums[index].toFixed(2) + ' 元';
        } else {
          sums[index] = '';
        }
      });
      return sums;
    },

    getBudgetInfo(id) {
      let find = {}
      for (let i = 0; i < this.departmentList.length; i++) {
        let children = this.departmentList[i]?.childrenList || []
        for (let j = 0; j < children.length; j++) {
          if (children[j].id == id) {
            find = children[j]
            break
          }
        }
      }
      return find
    },
    //提交校验
    submitCheck(businessStatus,id){
      return new Promise((resolve,reject)=>{
        const formRef = this.$refs.infoForm;
        const infoForm = this.infoForm;
        let param = {};
        let flag = false;
        // 草稿：跳过所有校验，直接组装参数保存
        if (businessStatus == 'draft') {
          const draftParam = this.buildDraftParam();
          resolve({flag: true, param: draftParam});
          return;
        }
        formRef.validate(async valid => {
          if (valid) {
            // return;
            // let taxInfo = infoForm.taxInfoList
            let expenseBudgetListTotalMoney = 0, expenseDetailListTotalMoney = 0, expenseInAccountInfoListTotalMoney = 0
            param = {
              data: {
                projectId: infoForm.projectId,
                expenseProjectName:infoForm.expenseProjectName,
                companyId: infoForm.companyId,
                expenseCompanyId: infoForm.expenseCompanyId,
                expenseCompanyName: infoForm.expenseCompanyName,
                expenseUserName:infoForm.expenseUserName,
                companyNumber: this.companyNumber,
                attachmentCount: infoForm.attachmentCount,
                remark: infoForm.remark,
                businessStatus: businessStatus == 'draft' ? '0' : '1',
                      assetsProcessVoList:this.infoForm.assetsProcessVoList || [],
                accountDetailedVoList: this.infoForm.accountDetailedVoList.map((item, index) => {
                  item.sort = index
                  return {
                    expenseReimbursementId: item.expenseReimbursementId,
                    payMoney: item.payMoney,
                    flowName: item.flowName,
                    processId: item.processId,
                    alreadyMoney: item.alreadyMoney,
                    notMoney: item.notMoney,
                    thisMoney: item.thisMoney,
                  }
                })
              },
              expenseBudgetList: infoForm.expenseBudgetList.map((x, index) => {
                let budgetId = x.allChildId.length > 1 ? x.allChildId[x.allChildId.length - 1] : ''
                expenseBudgetListTotalMoney = math.add(expenseBudgetListTotalMoney, Number(x.money))
                // console.log('expenseBudgetListTotalMoney',expenseBudgetListTotalMoney)
                const obj = {
                  companyNumber: x.companyNumber,
                  allChildId: x.allChildId.join(),
                  budgetId,// x.allChildId.length > 1 ? x.allChildId[x.allChildId.length - 1] : '',
                  mainId: x.allChildId[1],
                  departmentId: x.allChildId[0],
                  money: x.money,
                  type: 2,//: infoForm.projectId ? 1 : 2 ,// 选了项目传1
                  remark: x.remark,
                  sort: index,
                };
                //根据归口id查找归口信息，判断是项目归口还是公司归口
                let find = this.getBudgetInfo(budgetId)
                if (find.isProject && find.projectId) {
                  obj.type = 1
                  obj.projectId = find.projectId
                }
                if (this.operaType == 'edit') {
                  obj.id = x.id;
                }
                return obj;
              }),
              taxInfoList: infoForm.taxInfoList.map((el, index) => {
                el.sort = index
                return el
              }),
              expenseDetailList: infoForm.expenseDetailList.map((x, index) => {
                expenseDetailListTotalMoney = math.add(expenseDetailListTotalMoney, Number(x.money))
                const obj = {
                  type: x.type,
                  typeId:x.typeId,
                  sing:x.sing,
                  remark: x.remark,
                  money: x.money,
                  depId:x.depId,
                  depName:x.depName,
                  // attachmentIds: x.uploadFileList.map(x => x.fileId).join(','),
                  attachmentIds: x.uploadFileList.map(x => x.id).join(','),
                  invoiceType: x.invoiceType,
                  sort: index
                }
                if (this.operaType == 'edit') {
                  obj.id = x.id;
                }
                return obj;
              }),
              expenseInAccountInfoList: this.filteredAccountInfoList.map((item, index) => {
                expenseInAccountInfoListTotalMoney = math.add(expenseInAccountInfoListTotalMoney, Number(item.money || 0))
                item.sort = index
                return item
              })
            };
            // infoForm.repay != 'repayRequest'
            if (this.infoForm.repay != 'repayRequest' && Number(expenseBudgetListTotalMoney) != Number(expenseDetailListTotalMoney)) {
              this.$parent.$parent.$parent.submitLoading = false
              this.$message.error('费用预算类型金额合计与费用明细金额合计不一致')
              resolve({flag:false})
            }
            // if (expenseInAccountInfoListTotalMoney > 0) {//入账信息
            if (Number(expenseInAccountInfoListTotalMoney) != Number(expenseDetailListTotalMoney)) {
              this.$parent.$parent.$parent.submitLoading = false
              this.$message.error('入账信息金额与费用明细金额合计不一致')
              resolve({flag:false})
            }
            // }
            if (this.infoForm.repay && !this.infoForm?.accountDetailedVoList?.length) {
              this.$parent.$parent.$parent.submitLoading = false
              this.$message.error('请选择请款单或者借款单')
              resolve({flag:false})
            }
            // 冲请款时校验所有选中请款流程的收款人是否一致
            if (this.infoForm.repay == 'repayRequest' && this.infoForm?.accountDetailedVoList?.length > 1) {
              const payeeList = this.infoForm.accountDetailedVoList.map(item => item.payCompanyName || '')
              const firstPayee = payeeList[0]
              const allSame = payeeList.every(name => name === firstPayee)
              if (!allSame) {
                this.$parent.$parent.$parent.submitLoading = false
                this.$message.error('关联请款单中的请款流程收款人不一致，请检查后重新选择')
                resolve({flag:false})
                return
              }
            }
            if (this.infoForm.repay == 'repayRequest') {
              param.expenseBudgetList = []
            }
            if (this.operaType == 'edit') {
              param.data.id = this.id;
            }
            //检测报销单位 先去掉
            let checkData = {
              name:infoForm.expenseInAccountInfoList[0].name,
              company:{
                id:this.infoForm.expenseCompanyId
              }
            }
            //检测报销单位 先去掉 后面要开放
            // if(!this.isDisabled('accountInfoName')){
            //   try {
            //     let checkRes = await this.$axios.post(Api.budgetManage.verifyName,{data:checkData})
            //     if(checkRes?.isSuccess){
            //       flag = true
            //       resolve({flag,param})
            //     }else{
            //       this.$message.error('【入账信息】单位或姓名不存在')
            //       resolve({flag:false})
            //     }
            //   } catch (error) {
            //     resolve({flag:false})
            //   }
            // }else{
              flag = true
              resolve({flag,param})
            // }

          } else {
            this.$message.error('有必填项未填')
            this.$parent.$parent.$parent.submitLoading = false
            resolve({flag:false})
          }
        });
      })
    },
    buildDraftParam() {
      const infoForm = this.infoForm;
      const param = {
        data: {
          projectId: infoForm.projectId,
          expenseProjectName: infoForm.expenseProjectName,
          companyId: infoForm.companyId,
          expenseCompanyId: infoForm.expenseCompanyId,
          expenseCompanyName: infoForm.expenseCompanyName,
          expenseUserName: infoForm.expenseUserName,
          companyNumber: this.companyNumber,
          attachmentCount: infoForm.attachmentCount,
          remark: infoForm.remark,
          businessStatus: '0',
          assetsProcessVoList: this.infoForm.assetsProcessVoList || [],
          accountDetailedVoList: this.infoForm.accountDetailedVoList.map((item, index) => {
            item.sort = index;
            return {
              expenseReimbursementId: item.expenseReimbursementId,
              payMoney: item.payMoney,
              flowName: item.flowName,
              processId: item.processId,
              alreadyMoney: item.alreadyMoney,
              notMoney: item.notMoney,
              thisMoney: item.thisMoney,
            };
          })
        },
        expenseBudgetList: infoForm.expenseBudgetList.map((x, index) => {
          let budgetId = x.allChildId.length > 1 ? x.allChildId[x.allChildId.length - 1] : '';
          const obj = {
            companyNumber: x.companyNumber,
            allChildId: x.allChildId.join(),
            budgetId,
            mainId: x.allChildId[1],
            departmentId: x.allChildId[0],
            money: x.money,
            type: 2,
            remark: x.remark,
            sort: index,
          };
          let find = this.getBudgetInfo(budgetId);
          if (find.isProject && find.projectId) {
            obj.type = 1;
            obj.projectId = find.projectId;
          }
          if (this.operaType == 'edit') {
            obj.id = x.id;
          }
          return obj;
        }),
        taxInfoList: infoForm.taxInfoList.map((el, index) => {
          el.sort = index;
          return el;
        }),
        expenseDetailList: infoForm.expenseDetailList.map((x, index) => {
          const obj = {
            type: x.type,
            typeId: x.typeId,
            sing: x.sing,
            remark: x.remark,
            money: x.money,
            depId: x.depId,
            depName: x.depName,
            attachmentIds: x.uploadFileList.map(x => x.id).join(','),
            invoiceType: x.invoiceType,
            sort: index
          };
          if (this.operaType == 'edit') {
            obj.id = x.id;
          }
          return obj;
        }),
        expenseInAccountInfoList: this.filteredAccountInfoList.map((item, index) => {
          item.sort = index;
          return item;
        })
      };

      if (this.infoForm.repay == 'repayRequest') {
        param.expenseBudgetList = [];
      }
      if (this.operaType == 'edit') {
        param.data.id = this.id;
      }
      return param;
    },

    // 其他操作
    async submit(businessStatus,batchCode) {
      let res = await this.submitCheck(businessStatus)
      if (res.flag) {
        let data = res.param
        data.batchCode = batchCode
        return this.postData(data);
      }
    },

    postData(data) {
      return this.$axios.post(Api.budgetManage.expenseReimbursementSave, data);
    },
    handleChange(value, index) {
      let thisDepartmentList = this.infoForm.expenseBudgetList[index].departmentList
      thisDepartmentList.forEach(item => {
        const childrenList = item?.childrenList || []
        childrenList.forEach(it => {
          this.$set(it, 'disabled', false)
        })
      })
      this.infoForm.expenseBudgetList.forEach(el => {
        let allChildId = el?.allChildId || []
        let departmentId = allChildId[0]
        let childId = allChildId[1]
        let find = thisDepartmentList.find(item => item.id == departmentId)
        // console.log('find',find)
        if (find && find.childrenList) {
          // {{ scope.row.departmentName }} / {{ scope.row.budgetName }}
          el.departmentName = find.name
          let childFind = find.childrenList.find(item => item.id == childId)
          // console.log('childFind',childFind)
          if (childFind) {
            this.$set(childFind, 'disabled', true)
            el.budgetName = childFind.name
          }
        }
      })
    },
    filterWithdraw(data) {
      const len = data.length || 0;
      const arr = [];
      for (let i = len - 1; i >= 0; i--) {
        if (data[i].auditStatus == 'withdraw') break;
        arr.unshift(data[i]);
      }
      return arr;
    },
    getPostScriptList() {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.approveManage.getPostScriptList,
          {
            data: {
              flowInstanceId: this.$attrs.flowInstanceId//this.flowInstanceId
            }
          },
          (res) => {
            if (res.isSuccess) {
              this.postscriptList = this.generateTree(res.data);
              // this.postscriptList = res.data;
              resolve()
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    generateTree(flatArray) {
      // 创建一个映射，用于存储每个节点的引用
      const nodeMap = {};
      // 创建一个数组，用于存储树的根节点
      const tree = [];

      // 遍历扁平数组，初始化每个节点
      flatArray.forEach(item => {
        nodeMap[item.id] = { ...item, children: [] };
      });

      // 再次遍历扁平数组，构建树结构
      flatArray.forEach(item => {
        const node = nodeMap[item.id];
        if (item.pid === null) {
          // 如果没有父节点，则为根节点
          tree.push(node);
        } else {
          // 如果有父节点，则将当前节点添加到父节点的子节点数组中
          const parentNode = nodeMap[item.pid];
          if (parentNode) {
            node.isReplay = true;
            parentNode.children.push(node);
          }
        }
      });
      return tree;
    },
    // 已建任务和流程日志操作状态字符转换
    translateStatus(obj) {
      let chnStatus;
      if (obj.auditStatus) {
        switch (obj.auditStatus) {
          case 'pass':
            chnStatus = '通过';
            break;
          case 'no_pass':
            chnStatus = '驳回';
            break;
          case 'withdraw':
            chnStatus = '撤销';
            break;
          case 'retrieve':
            chnStatus = '取回';
            break;
          case 'transfer':
            chnStatus = '移交';
            break;
          case 'roll_back_the_previous_level':
            chnStatus = '回退上一节点';
            break;
          default:
            chnStatus = '';
            break;
        }
      } else if (obj.flowStatus) {
        switch (obj.flowStatus) {
          case 'await_sent':
            chnStatus = '待发';
            break;

          case 'run':
            chnStatus = '运行中';
            break;

          case 'withdraw':
            chnStatus = '撤销';
            break;

          case 'termination':
            chnStatus = '终止';
            break;

          case 'rejected':
            chnStatus = '驳回';
            break;

          case 'end':
            chnStatus = '完结';
            break;

          default:
            chnStatus = '';
            break;
        };
      }
      obj.auditStatus = chnStatus;
    },
  }
};

</script>
<style lang='scss' scoped>
$bc: rgb(153, 153, 153);

.outer {
  // padding: 10px;
  // overflow: hidden;
  background: white;
  display: flow-root;
  height: 100%;
  width: 1100px;
  margin: 0 auto;
  max-width: 1136px;
  font-size:16px;
  // .top {
  //   margin: 40px 0 0 40px;
  // }
}


// @media screen and (min-width: 1300px) {
//   .outer {
//     width: 80%;

//   }
// }

.show-flag-list {
  font-size: 14px;
  float: right;
  cursor: pointer;
  transition: all 0.3s ease;
  margin-right:5px;
  i {
    transition: all 0.3s ease;
  }
}

.show-content {
  transform: rotate(-90deg);
}

#budgetTypeTable,
#expendDetailTable,
#valueAddedTaxTable {
  ::v-deep .el-form-item--mini.el-form-item {
    margin-bottom: 0px;
  }

  ::v-deep .el-table__footer-wrapper .cell {
    white-space: pre-line;
  }
}

// #expendDetailTable {
//   ::v-deep .el-table__footer-wrapper .cell {
//     white-space: pre-line;
//   }
// }
::v-deep #expendForm .el-table .el-table__body-wrapper .cell {
  min-height: 30px;
}

::v-deep #expendForm {
  .el-row {
    border-bottom: 1px solid $bc;
  }

  .el-card {
    border-radius: 0;
    border-color: $bc;
    border-bottom:none;
    .el-card__body {
      padding: 0;
    }
  }

  .el-form-item__label {
    line-height: normal;
    padding: 5px 1px;
    // border-right: 1px solid rgb(153,153,153);
    // border-bottom: 1px solid rgb(153,153,153);
    text-align: center;
    flex-shrink: 0;
    font-size:16px;
    color:#303133;
    font-weight:initial;
  }

  .el-form-item__content {
    border-left: 1px solid $bc;
    padding: 2px 8px;
    margin-left: 0 !important;
    width: 100%;
    display: flex;
    align-items: center;
    padding:1px;
    font-size:16px;
  }

  .el-form-item--mini.el-form-item {
    margin-bottom: 0;
    border-right: 1px solid $bc;
  }
  .repay-table .el-form-item--mini.el-form-item {
    border-right:none;
  }
  .el-form-item.el-form-item--mini {
    display: flex;
    align-items: stretch;
  }

  .sub-title {
    text-align: center;
    padding: 2px 0;
    font-weight: 600;
    // border-bottom: 1px solid $bc;
    // background:#e3e3e3;
  }
  .sub-title-button{
    padding:3px 0 3px 5px;
    // padding-left:5px;
    border-top:1px solid $bc;
  }
  .el-table--group,
  .el-table--border {
    border-color: $bc;
    border-left: none;
  }

  .el-table th.el-table__cell.is-leaf,
  .el-table td.el-table__cell {
    border-color: $bc;
    font-size:16px;
  }

  .el-table__body .el-form-item__content {
    border: none;
  }

  .box-card .el-form-item--mini.el-form-item {
    border: none;
  }

  .el-table__footer-wrapper tbody td.el-table__cell {
    border-color: $bc;
  }

  .el-table::before {
    background: $bc;
  }


  .el-button--mini{
    padding:4px 10px;
  }
  .accountinform .el-form-item__content{
    padding:0;
    border-left:none;
    .el-table::before{
      height:0;
    }
    .el-table--border{
      border:none;
    }
  }
  .el-table--mini .el-table__cell{
    padding:0;
    text-align:center;
    padding:2px 0;

  }
  .el-table__footer-wrapper tbody td.el-table__cell{
    background:#fff;
  }
  .el-table .cell {
    padding-right:0;
    padding-left:0;
  }
  // thead.has-gutter .cell{
  //   padding-left:2px;
  // }
  .el-textarea .el-input__count{
    background:transparent;
  }
  .el-table__empty-block{
    display:none;
  }
  .el-radio-group{
    height:28px;
    display:flex;
    align-items:center;
  }
  .el-input--mini{
    font-size:16px;
    height:100%;
    .el-input__inner{
      padding-left:5px;
      height:100%;
      line-height:normal;
      text-align:center;
    }
  }
  .action .cell{
    display:flex;
    align-items:center;
    justify-content:center;
  }
  .el-cascader--mini{
    font-size:16px;
  }
  .el-cascader-panel .el-cascader-node__label{
    font-size:16px !important;
  }
  .el-textarea__inner{
    padding:5px;
    text-align:center;
  }
  .el-table__header-wrapper th{
    font-weight:initial;
  }
  .expenseCompany .el-input__inner{
    padding-right:15px;
    padding-left:1px;
  }
  .expenseCompany .el-input__suffix{
    right:0;
  }
  .el-table th.el-table__cell.is-leaf, .el-table td.el-table__cell{
    color: #000;
  }
  .repay-table th{
    border-top: 1px solid $bc;
  }
}
.escaDisable{
  background: #F5F7FA;
}
::v-deep .dytable-view-container{
  padding: 0;
}
// ::v-deep #expendForm .el-table .cell {
//   white-space: nowrap;
// }
// formMaking表单打印样式
@media print {

  // 流程样式
  ::v-deep {
    .postscript-divWrap {
      width: 900px;
      margin: 0 auto;

      .flow-wrap {
        width: 900px !important;
      }
    }

    .flow-log-container {
      margin: 0 auto;
      overflow: initial;

      .flowWrap {
        margin-top: 0px !important;
        padding: 0px;
      }
    }

    // .script-item-child {
    //   margin: 2px 0;
    //   padding: 4px;
    //   margin-left: 8px;
    //   margin-bottom: 5px;
    //   background: aliceblue;
    //   border: 1px solid #ccc;
    //   .item-info-child {
    //     position: relative;
    //     color: #8c8c8c;
    //     .item-info-date {
    //       margin-left: 15px;
    //     }
    //   }
    // }
  }
}
</style>

