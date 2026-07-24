@isTest
private class AccountServiceTest {
    @isTest
    static void filtersActiveAccounts() {
        List<Account> input = new List<Account>{
            new Account(Name = 'Acme', Status__c = 'Active'),
            new Account(Name = 'Globex', Status__c = 'Inactive')
        };

        Test.startTest();
        List<Account> active = AccountService.activeAccounts(input);
        Test.stopTest();

        System.assertEquals(1, active.size(), 'Solo se espera una cuenta activa');
        System.assertEquals('Acme', active[0].Name, 'La cuenta activa debe ser Acme');
    }
}
